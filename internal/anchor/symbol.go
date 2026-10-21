package anchor

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

var symbolName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`)

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
	fset := token.NewFileSet()
	f, _ := parser.ParseFile(fset, "", data, parser.SkipObjectResolution)
	if f == nil {
		return 0, 0, false
	}
	span := func(n ast.Node) (int, int, bool) {
		return fset.Position(n.Pos()).Line, fset.Position(n.End()).Line, true
	}
	recv, method, isMethod := strings.Cut(name, ".")
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if isMethod && d.Recv != nil && d.Name.Name == method && recvType(d.Recv) == recv {
				return span(d)
			}
			if !isMethod && d.Recv == nil && d.Name.Name == name {
				return span(d)
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
						return span(d) // include the keyword line of a single declaration
					}
					return span(s)
				}
			}
		}
	}
	return 0, 0, false
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
