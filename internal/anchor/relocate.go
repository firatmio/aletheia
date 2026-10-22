package anchor

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Limits for the project scan behind Relocate.
const maxGoFiles = 5000

// skipDirs are never searched for a moved declaration: dependencies, build
// output and test fixtures. Hidden directories are skipped as well.
var skipDirs = map[string]bool{
	"node_modules": true, "vendor": true, "dist": true, "build": true, "target": true,
	"testdata": true, "bin": true, "obj": true,
}

// Moved is where a sealed symbol anchor's declaration is now.
type Moved struct {
	Path          string
	Line, EndLine int
}

// sealSymbol adds the declaration's fingerprint to a symbol anchor in a Go
// file that holds. Other languages stay unsealed: their declarations are
// found by one line, too little to recognise moved code by.
func sealSymbol(root string, a Anchor) Anchor {
	a.Seal = ""
	if !strings.HasSuffix(a.Path, ".go") {
		return a
	}
	full, err := resolve(root, a.Path)
	if err != nil {
		return a
	}
	data, err := readHead(full)
	if err != nil {
		return a
	}
	if fp, _, _, ok := goFingerprint(data, a.Arg); ok {
		a.Seal = fp
	}
	return a
}

// Files is a scan of a project's Go files, made by GoFiles: regular files
// under the root, found without following symlinks, so reading them never
// leaves the project. On first use it indexes which top-level names each
// file declares, so relocating several anchors costs one pass.
type Files struct {
	root  string
	paths []string
	decls map[string][]string
}

// GoFiles lists the project's Go files for Relocate. Hidden, dependency and
// fixture directories and symlinks are skipped.
func GoFiles(root string) *Files {
	f := &Files{root: root}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != root && (strings.HasPrefix(d.Name(), ".") || skipDirs[d.Name()]) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		if len(f.paths) == maxGoFiles {
			return filepath.SkipAll
		}
		if rel, err := filepath.Rel(root, p); err == nil {
			f.paths = append(f.paths, filepath.ToSlash(rel))
		}
		return nil
	})
	return f
}

func (f *Files) read(rel string) ([]byte, error) {
	h, err := os.Open(filepath.Join(f.root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, err
	}
	defer h.Close()
	data, err := io.ReadAll(io.LimitReader(h, maxScanBytes))
	return normalizeText(data), err
}

// declaring returns the files that declare name (Name or Type.Method).
func (f *Files) declaring(name string) []string {
	if f.decls == nil {
		f.decls = map[string][]string{}
		for _, rel := range f.paths {
			data, err := f.read(rel)
			if err != nil {
				continue
			}
			for _, n := range declaredNames(data) {
				f.decls[n] = append(f.decls[n], rel)
			}
		}
	}
	return f.decls[name]
}

// declaredNames lists a Go file's top-level names as symbol anchors spell
// them: Name for functions, types, vars and consts, Type.Method for methods.
func declaredNames(data []byte) []string {
	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, "", data, parser.SkipObjectResolution)
	if file == nil {
		return nil
	}
	var out []string
	for _, d := range file.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				out = append(out, d.Name.Name)
			} else if r := recvType(d.Recv); r != "" {
				out = append(out, r+"."+d.Name.Name)
			}
		case *ast.GenDecl:
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *ast.TypeSpec:
					out = append(out, s.Name.Name)
				case *ast.ValueSpec:
					for _, n := range s.Names {
						out = append(out, n.Name)
					}
				}
			}
		}
	}
	return out
}

// Relocate looks for the declaration of a sealed Go symbol anchor whose own
// file no longer has it. It reports the new place only when exactly one
// declaration with that name in files has the sealed fingerprint: the same
// code, moved. Anything less certain is not a move. Changed lists the files
// that declare the name with a different fingerprint (moved and edited, or
// another declaration with the same name), as a hint for a human.
func Relocate(a Anchor, files *Files) (moved Moved, changed []string, ok bool) {
	if a.Kind != KindSymbol || a.Seal == "" || !strings.HasSuffix(a.Path, ".go") || files == nil {
		return Moved{}, nil, false
	}
	var matches []Moved
	for _, rel := range files.declaring(a.Arg) {
		if rel == a.Path {
			continue
		}
		data, err := files.read(rel)
		if err != nil {
			continue
		}
		fp, start, end, found := goFingerprint(data, a.Arg)
		switch {
		case !found:
		case fp == a.Seal:
			matches = append(matches, Moved{Path: rel, Line: start, EndLine: end})
		default:
			changed = append(changed, rel)
		}
	}
	if len(matches) != 1 {
		return Moved{}, changed, false
	}
	return matches[0], changed, true
}
