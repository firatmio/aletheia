// Package codemap builds a ranked, token-budgeted map of a project's
// declarations, so an agent can see where things live before it reads any
// file. Go is parsed with go/parser; other languages are scanned for
// declaration keywords. The map is computed from the files on every call:
// there is nothing stored that could go stale.
package codemap

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/firatmio/aletheia/internal/search"
)

// Limits that keep a build fast on large repositories.
const (
	maxFiles     = 5000
	maxFileBytes = 1 << 20
	maxSymbols   = 40 // rendered at most
	maxPerFile   = 6  // so one big file cannot crowd out the others
)

// Symbol is one declaration.
type Symbol struct {
	Path    string // slash-separated, relative to the project root
	Name    string // Name, or Type.Method for Go methods
	Line    int
	EndLine int
	Doc     string // first line of the doc comment (Go only)
	refs    int    // other files that mention the name
}

// Map is the set of declarations in a project.
type Map struct {
	Symbols    []Symbol
	Files      int      // source files scanned
	BuildFiles []string // manifests, build scripts and CI config, e.g. go.mod, .github/workflows/ci.yml
}

// buildNames are files that say how a project is built, tested or shipped.
// They have no declarations, so they would never show up in the map, yet
// they are the answer to "how is this built".
var buildNames = map[string]bool{
	"go.mod": true, "package.json": true, "cargo.toml": true, "pyproject.toml": true, "setup.py": true,
	"requirements.txt": true, "pom.xml": true, "build.gradle": true, "build.gradle.kts": true, "gemfile": true,
	"cmakelists.txt": true, "makefile": true, "justfile": true, "taskfile.yml": true, "dockerfile": true,
	"docker-compose.yml": true, "docker-compose.yaml": true, "compose.yaml": true,
	".gitlab-ci.yml": true, ".goreleaser.yml": true, ".goreleaser.yaml": true,
}

// buildWords in a prompt ask about building, testing or shipping.
var buildWords = map[string]bool{}

func init() {
	for _, w := range search.Tokenize("build builds building built compile compiling test tests testing ci cd " +
		"deploy deploying deployment release releasing install installing lint linting pipeline workflow workflows " +
		"docker makefile dependencies derle derleme kurulum") {
		buildWords[w] = true
	}
}

const maxBuildFiles = 12

// SuggestedBudget sizes a map to the project: a small project gets none,
// since an agent reads all of it in a few calls anyway, and the budget grows
// with the number of source files.
func (m *Map) SuggestedBudget() int {
	switch {
	case m.Files < 25:
		return 0
	case m.Files < 150:
		return 300
	default:
		return 600
	}
}

var skipDirs = map[string]bool{
	"node_modules": true, "vendor": true, "dist": true, "build": true, "target": true,
	"testdata": true, "__pycache__": true, "venv": true, "bin": true, "obj": true,
}

var sourceExt = map[string]bool{
	".go": true, ".py": true, ".js": true, ".jsx": true, ".mjs": true, ".ts": true, ".tsx": true,
	".java": true, ".kt": true, ".rs": true, ".rb": true, ".php": true, ".cs": true, ".swift": true,
	".c": true, ".h": true, ".cc": true, ".cpp": true, ".hpp": true, ".scala": true,
}

var (
	identRE = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]{2,}`)
	declRE  = regexp.MustCompile(`(?m)^([ \t]*)(?:(?:export|pub|async|static|public|private|protected|abstract|default)[ \t]+)*` +
		`(?:def|class|function|func|fn|interface|struct|enum|trait|type)[ \t]+([A-Za-z_][A-Za-z0-9_]*)`)
)

// Build scans the project under root. Hidden directories, dependency and
// build directories, symlinks and files over 1 MiB are skipped.
func Build(root string) (*Map, error) {
	m := &Map{}
	mentions := map[string]int{} // identifier -> number of files containing it
	files := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entries are skipped, not fatal
		}
		name := d.Name()
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if p != root && (strings.HasPrefix(name, ".") || skipDirs[name]) {
				return filepath.SkipDir
			}
			if rel == "scripts" {
				m.BuildFiles = append(m.BuildFiles, "scripts/")
			}
			return nil
		}
		if buildNames[strings.ToLower(name)] && strings.Count(rel, "/") <= 1 && d.Type()&fs.ModeSymlink == 0 {
			m.BuildFiles = append(m.BuildFiles, rel)
		}
		if d.Type()&fs.ModeSymlink != 0 || !sourceExt[strings.ToLower(filepath.Ext(name))] {
			return nil
		}
		if files >= maxFiles {
			return filepath.SkipAll
		}
		info, err := d.Info()
		if err != nil || info.Size() > maxFileBytes {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil || generated(data) {
			return nil
		}
		files++
		if strings.HasSuffix(rel, ".go") {
			m.Symbols = append(m.Symbols, goSymbols(rel, data)...)
		} else {
			m.Symbols = append(m.Symbols, lineSymbols(rel, data)...)
		}
		seen := map[string]bool{}
		for _, id := range identRE.FindAll(data, -1) {
			if s := string(id); !seen[s] {
				seen[s] = true
				mentions[s]++
			}
		}
		return nil
	})
	m.Files = files
	// Hidden directories are skipped above; CI workflows are the exception.
	for _, pattern := range []string{".github/workflows/*.yml", ".github/workflows/*.yaml"} {
		found, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern)))
		for _, f := range found {
			if rel, err := filepath.Rel(root, f); err == nil {
				m.BuildFiles = append(m.BuildFiles, filepath.ToSlash(rel))
			}
		}
	}
	for i := range m.Symbols {
		s := &m.Symbols[i]
		name := s.Name
		if _, method, ok := strings.Cut(name, "."); ok {
			name = method
		}
		s.refs = max(mentions[name]-1, 0)
	}
	return m, err
}

// generated reports whether a file carries the standard "Code generated ...
// DO NOT EDIT." header near its top.
func generated(data []byte) bool {
	head := data[:min(len(data), 2048)]
	return bytes.Contains(head, []byte("Code generated")) && bytes.Contains(head, []byte("DO NOT EDIT"))
}

func goSymbols(rel string, data []byte) []Symbol {
	fset := token.NewFileSet()
	f, _ := parser.ParseFile(fset, "", data, parser.ParseComments|parser.SkipObjectResolution)
	if f == nil {
		return nil
	}
	test := strings.HasSuffix(rel, "_test.go")
	var out []Symbol
	add := func(name string, n ast.Node, doc *ast.CommentGroup) {
		s := Symbol{Path: rel, Name: name, Line: fset.Position(n.Pos()).Line, EndLine: fset.Position(n.End()).Line}
		if doc != nil {
			s.Doc, _, _ = strings.Cut(strings.TrimSpace(doc.Text()), "\n")
		}
		out = append(out, s)
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if test && !strings.HasPrefix(d.Name.Name, "Test") {
				continue // helpers in tests are noise; tests themselves show where to add one
			}
			name := d.Name.Name
			if d.Recv != nil && len(d.Recv.List) > 0 {
				name = recvName(d.Recv.List[0].Type) + "." + name
			}
			add(name, d, d.Doc)
		case *ast.GenDecl:
			if test {
				continue
			}
			for _, sp := range d.Specs {
				doc := d.Doc
				switch sp := sp.(type) {
				case *ast.TypeSpec:
					if sp.Doc != nil {
						doc = sp.Doc
					}
					add(sp.Name.Name, specOrDecl(d, sp), doc)
				case *ast.ValueSpec:
					for _, n := range sp.Names {
						if n.IsExported() { // unexported vars and consts are mostly noise
							add(n.Name, specOrDecl(d, sp), doc)
						}
					}
				}
			}
		}
	}
	return out
}

func specOrDecl(d *ast.GenDecl, s ast.Spec) ast.Node {
	if len(d.Specs) == 1 {
		return d
	}
	return s
}

func recvName(t ast.Expr) string {
	if s, ok := t.(*ast.StarExpr); ok {
		t = s.X
	}
	switch x := t.(type) {
	case *ast.IndexExpr:
		t = x.X
	case *ast.IndexListExpr:
		t = x.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name
	}
	return "?"
}

func lineSymbols(rel string, data []byte) []Symbol {
	var out []Symbol
	for _, loc := range declRE.FindAllSubmatchIndex(data, -1) {
		line := bytes.Count(data[:loc[0]], []byte("\n")) + 1
		out = append(out, Symbol{Path: rel, Name: string(data[loc[4]:loc[5]]), Line: line, EndLine: line})
	}
	return out
}

// splitWords turns "validateClampFunc" and "fold_test.go" into searchable words.
func splitWords(s string) string {
	var b strings.Builder
	prev := rune(0)
	for _, r := range s {
		if unicode.IsUpper(r) && unicode.IsLower(prev) {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
		prev = r
	}
	return b.String()
}

func words(s string) []string { return search.Tokenize(splitWords(s)) }

func nameMatches(name string, query map[string]bool) int {
	n := 0
	for _, w := range words(name) {
		if query[w] {
			n++
		}
	}
	return n
}

// Render returns the best files for query with their most relevant
// declarations, within an estimated token budget:
//
//	builtin/builtin.go: Builtins 35-1081, Function 10-30
//
// Files are ranked first (BM25 over the path, weighted three times, plus
// declaration names and doc lines), as in Aider's repo map, so a folder
// named in the prompt beats scattered symbols that share a common word.
// Within a file, declarations named in the prompt come first, then the
// ones other files mention most. Without a query, files whose declarations
// are mentioned most come first.
func (m *Map) Render(query string, budget int) string {
	byFile := map[string][]Symbol{}
	var files []string
	for _, s := range m.Symbols {
		if _, ok := byFile[s.Path]; !ok {
			files = append(files, s.Path)
		}
		byFile[s.Path] = append(byFile[s.Path], s)
	}
	q := map[string]bool{}
	weights := map[string]float64{} // a word the prompt repeats is its topic: "builtin ... builtins"
	for _, w := range words(query) {
		if !search.IsStopword(w) {
			q[w] = true
			weights[w]++
		}
	}

	score := map[string]float64{}
	if len(weights) == 0 && strings.TrimSpace(query) != "" {
		return "" // nothing in the prompt says where to look
	}
	var b strings.Builder
	used := 0
	if line := m.buildLine(weights); line != "" {
		b.WriteString(line + "\n")
		used += search.EstimateTokens(line) + 1
	}
	if len(weights) > 0 {
		docs := make([]search.Doc, len(files))
		for i, f := range files {
			p := words(path.Dir(f) + " " + strings.TrimSuffix(path.Base(f), path.Ext(f)))
			t := append(append(append([]string{}, p...), p...), p...)
			for _, s := range byFile[f] {
				t = append(append(t, words(s.Name)...), search.Tokenize(s.Doc)...)
			}
			docs[i] = search.Doc{ID: f, Terms: t}
		}
		score = search.NewIndex(docs).WeightedScores(weights)
	} else {
		for _, f := range files {
			for _, s := range byFile[f] {
				score[f] += math.Log1p(float64(s.refs))
			}
		}
	}
	for f := range score {
		if strings.HasSuffix(f, "_test.go") || strings.Contains(f, "/test/") || strings.HasPrefix(f, "test/") {
			score[f] /= 2 // tests and test fixtures: useful, but rarely the place to start
		}
	}
	sort.SliceStable(files, func(i, j int) bool { return score[files[i]] > score[files[j]] })

	shown := 0
	for _, f := range files {
		if score[f] <= 0 || shown >= maxSymbols {
			break
		}
		syms := byFile[f]
		sort.SliceStable(syms, func(i, j int) bool {
			mi, mj := nameMatches(syms[i].Name, q), nameMatches(syms[j].Name, q)
			if mi != mj {
				return mi > mj
			}
			return syms[i].refs > syms[j].refs
		})
		var items []string
		cost := search.EstimateTokens(f) + 1
		for _, s := range syms {
			if len(items) == maxPerFile {
				break
			}
			item := fmt.Sprintf("%s %d", s.Name, s.Line)
			if s.EndLine > s.Line {
				item = fmt.Sprintf("%s %d-%d", s.Name, s.Line, s.EndLine)
			}
			items = append(items, item)
			cost += search.EstimateTokens(item) + 1
		}
		if used+cost > budget && used > 0 {
			break
		}
		fmt.Fprintf(&b, "%s: %s\n", f, strings.Join(items, ", "))
		used += cost
		shown += len(items)
	}
	return strings.TrimRight(b.String(), "\n")
}

// buildLine lists the build and CI files for an overview (no query) or a
// prompt about building, testing or shipping:
//
//	build and CI: go.mod, Makefile, scripts/, .github/workflows/ci.yml
func (m *Map) buildLine(weights map[string]float64) string {
	if len(m.BuildFiles) == 0 {
		return ""
	}
	ask := len(weights) == 0
	for w := range weights {
		ask = ask || buildWords[w]
	}
	if !ask {
		return ""
	}
	files := m.BuildFiles
	more := ""
	if len(files) > maxBuildFiles {
		more = fmt.Sprintf(" (+%d more)", len(files)-maxBuildFiles)
		files = files[:maxBuildFiles]
	}
	return "build and CI: " + strings.Join(files, ", ") + more
}
