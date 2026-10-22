package anchor

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"regexp"
	"strings"
)

var symbolName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`)

// sealBytes is the length of a symbol fingerprint: 8 bytes, 16 hex digits.
const sealBytes = 8

func isSeal(s string) bool {
	if len(s) != 2*sealBytes {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && strings.ToLower(s) == s
}

// lineOf returns the 1-based line number of byte offset i.
func lineOf(data []byte, i int) int {
	return bytes.Count(data[:i], []byte("\n")) + 1
}

// findSymbol locates a top-level declaration by name and returns its line
// range. Go files are parsed with go/parser, so the range covers the whole
// declaration and Type.Method finds a method. Other languages are matched
// by a declaration keyword on one line, and only that line is reported.
func findSymbol(rel string, data []byte, name string) (int, int, bool) {
	if strings.HasSuffix(rel, ".go") {
		return findGoSymbol(data, name)
	}
	if _, method, ok := strings.Cut(name, "."); ok {
		name = method
	}
	re := regexp.MustCompile(`(?m)^[ \t]*(?:(?:export|pub|async|static|public|private|protected|abstract|default)[ \t]+)*` +
		`(?:def|class|function|func|fn|type|interface|struct|enum|trait|const|let|var|module)[ \t]+` +
		regexp.QuoteMeta(name) + `\b`)
	loc := re.FindIndex(data)
	if loc == nil {
		return 0, 0, false
	}
	line := lineOf(data, loc[0])
	return line, line, true
}

func findGoSymbol(data []byte, name string) (int, int, bool) {
	fset, n := goDecl(data, name)
	if n == nil {
		return 0, 0, false
	}
	return fset.Position(n.Pos()).Line, fset.Position(n.End()).Line, true
}

// goFingerprint returns the fingerprint of a top-level Go declaration and
// its line span. The doc comment is not part of it.
func goFingerprint(data []byte, name string) (fp string, start, end int, ok bool) {
	fset, n := goDecl(data, name)
	if n == nil {
		return "", 0, 0, false
	}
	from, to := fset.Position(n.Pos()), fset.Position(n.End())
	return fingerprint(data[from.Offset:to.Offset]), from.Line, to.Line, true
}

// fingerprint identifies a declaration's code independently of its layout:
// it hashes the Go tokens, without comments, semicolons and trailing commas,
// so gofmt, re-indenting or a changed comment keep the same fingerprint and
// any change to the code itself gives a new one.
func fingerprint(src []byte) string {
	fset := token.NewFileSet()
	var s scanner.Scanner
	s.Init(fset.AddFile("", fset.Base(), len(src)), src, nil, 0)
	var toks []string
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.SEMICOLON {
			continue
		}
		if (tok == token.RPAREN || tok == token.RBRACE || tok == token.RBRACK) && len(toks) > 0 && toks[len(toks)-1] == "," {
			toks = toks[:len(toks)-1]
		}
		if lit == "" {
			lit = tok.String()
		}
		toks = append(toks, lit)
	}
	sum := sha256.Sum256([]byte(strings.Join(toks, "\x1f")))
	return hex.EncodeToString(sum[:sealBytes])
}

// goDecl finds the top-level declaration of name (Name or Type.Method). A
// var, const or type in a group is its spec; alone, it is the whole
// declaration, keyword included.
func goDecl(data []byte, name string) (*token.FileSet, ast.Node) {
	fset := token.NewFileSet()
	f, _ := parser.ParseFile(fset, "", data, parser.SkipObjectResolution)
	if f == nil {
		return fset, nil
	}
	recv, method, isMethod := strings.Cut(name, ".")
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if isMethod && d.Recv != nil && d.Name.Name == method && recvType(d.Recv) == recv {
				return fset, d
			}
			if !isMethod && d.Recv == nil && d.Name.Name == name {
				return fset, d
			}
		case *ast.GenDecl:
			if isMethod {
				continue
			}
			for _, s := range d.Specs {
				var names []*ast.Ident
				switch s := s.(type) {
				case *ast.TypeSpec:
					names = []*ast.Ident{s.Name}
				case *ast.ValueSpec:
					names = s.Names
				}
				for _, n := range names {
					if n.Name != name {
						continue
					}
					if len(d.Specs) == 1 {
						return fset, d // include the keyword line of a single declaration
					}
					return fset, s
				}
			}
		}
	}
	return fset, nil
}

// recvType returns the type name of a method receiver: T, *T, T[K] or *T[K].
func recvType(fl *ast.FieldList) string {
	if fl == nil || len(fl.List) == 0 {
		return ""
	}
	t := fl.List[0].Type
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
	return ""
}
