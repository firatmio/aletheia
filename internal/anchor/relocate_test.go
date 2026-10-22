package anchor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeAll(t *testing.T, root, name, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fp(t *testing.T, src, name string) string {
	t.Helper()
	f, _, _, ok := goFingerprint([]byte(src), name)
	if !ok {
		t.Fatalf("%s not found in:\n%s", name, src)
	}
	return f
}

func TestFingerprintIgnoresLayoutButNotCode(t *testing.T) {
	base := "package p\n\n// Clamp limits x.\nfunc Clamp(x, lo, hi int) int {\n\tif x < lo {\n\t\treturn lo\n\t}\n\treturn min(x, hi)\n}\n"
	relaid := "package p\n\n// Another comment.\nfunc Clamp(x, lo, hi int) int { if x < lo { return lo }; return min(x, hi) } // trailing\n"
	if fp(t, base, "Clamp") != fp(t, relaid, "Clamp") {
		t.Fatal("comments and layout must not change the fingerprint")
	}
	if fp(t, base, "Clamp") == fp(t, strings.Replace(base, "min(x, hi)", "max(x, hi)", 1), "Clamp") {
		t.Fatal("a change to the code must change the fingerprint")
	}
	multi := "package p\n\nvar Names = []string{\n\t\"a\",\n\t\"b\",\n}\n"
	single := "package p\n\nvar Names = []string{\"a\", \"b\"}\n"
	if fp(t, multi, "Names") != fp(t, single, "Names") {
		t.Fatal("a trailing comma from gofmt must not change the fingerprint")
	}
}

// Stored seals are compared across versions: if this value changes, every
// sealed anchor in every memory store stops recognising moved code.
func TestFingerprintIsStable(t *testing.T) {
	if got := fp(t, "package builtin\n\nfunc Clamp() {}\n", "Clamp"); got != "7370a7fe3669256c" {
		t.Fatalf("fingerprint changed to %s; existing seals would no longer match", got)
	}
}

func TestSymbolAnchorsAreSealedForGoOnly(t *testing.T) {
	root := project(t)
	writeAll(t, root, "a.go", "package a\n\nfunc F() int { return 1 }\n")
	writeAll(t, root, "r.py", "def f():\n    pass\n")

	a, err := Make(root, "symbol:a.go::F@0000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	if !isSeal(a.Seal) || a.Seal == "0000000000000000" || !strings.HasSuffix(a.String(), "@"+a.Seal) {
		t.Fatalf("Make must compute the seal itself: %q", a.String())
	}
	if back, err := Parse(a.String()); err != nil || back != a {
		t.Fatalf("sealed anchor must round-trip: %+v %v", back, err)
	}
	if p, err := Make(root, "symbol:r.py::f"); err != nil || p.Seal != "" || strings.Contains(p.String(), "@") {
		t.Fatalf("non-Go symbols stay unsealed: %q %v", p.String(), err)
	}
	if _, err := Parse("symbol:a.go::F@xyz"); !errors.Is(err, ErrFormat) {
		t.Fatalf("a malformed seal must be rejected, got %v", err)
	}
}

func TestSealedSymbolHoldsAfterEditInPlace(t *testing.T) {
	root := project(t)
	writeAll(t, root, "a.go", "package a\n\nfunc F() int { return 1 }\n")
	a, err := Make(root, "symbol:a.go::F")
	if err != nil {
		t.Fatal(err)
	}
	writeAll(t, root, "a.go", "package a\n\n// F now returns two.\nfunc F() int { return 2 }\n")
	if r := Check(root, a); !r.OK {
		t.Fatalf("the seal is only for moves; an edit in place must still hold: %+v", r)
	}
}

func TestRelocateFindsTheSameCodeInAnotherFile(t *testing.T) {
	root := project(t)
	writeAll(t, root, "a.go", "package a\n\ntype T struct{}\n\nfunc (t *T) M() int { return 1 }\n\nfunc F(x int) int {\n\treturn x * 2\n}\n\nfunc G() {}\n")
	f, err := Make(root, "symbol:a.go::F")
	if err != nil {
		t.Fatal(err)
	}
	m, err := Make(root, "symbol:a.go::T.M")
	if err != nil {
		t.Fatal(err)
	}
	writeAll(t, root, "a.go", "package a\n\ntype T struct{}\n\nfunc G() {}\n")
	writeAll(t, root, "pkg/b.go", "package a\n\n// F doubles x, now in b.go.\nfunc F(x int) int { return x * 2 }\n\nfunc (t *T) M() int {\n\treturn 1\n}\n")

	if Check(root, f).OK {
		t.Fatal("F left a.go, so the anchor itself must fail")
	}
	got, _, ok := Relocate(f, GoFiles(root))
	if !ok || got != (Moved{Path: "pkg/b.go", Line: 4, EndLine: 4}) {
		t.Fatalf("F moved unchanged to pkg/b.go: %+v %v", got, ok)
	}
	if got, _, ok := Relocate(m, GoFiles(root)); !ok || got.Path != "pkg/b.go" || got.Line != 6 {
		t.Fatalf("method T.M moved unchanged to pkg/b.go: %+v %v", got, ok)
	}
	r, err := Reseal(root, f)
	if err != nil || r.Path != "pkg/b.go" || r.Seal != f.Seal || !Check(root, r).OK {
		t.Fatalf("reseal must record the new path: %+v %v", r, err)
	}
}

func TestRelocateRefusesWhatItCannotProve(t *testing.T) {
	root := project(t)
	src := "package a\n\nfunc F() int { return 1 }\n"
	writeAll(t, root, "a.go", src)
	a, err := Make(root, "symbol:a.go::F")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "a.go")); err != nil {
		t.Fatal(err)
	}

	writeAll(t, root, "b.go", "package a\n\nfunc F() int { return 2 }\n")
	if _, changed, ok := Relocate(a, GoFiles(root)); ok || len(changed) != 1 || changed[0] != "b.go" {
		t.Fatalf("moved and changed is not a move, but b.go is worth a hint: %v %v", changed, ok)
	}

	writeAll(t, root, "b.go", src)
	writeAll(t, root, "c.go", src)
	if _, _, ok := Relocate(a, GoFiles(root)); ok {
		t.Fatal("two identical copies: which one is meant cannot be known")
	}

	unsealed := a
	unsealed.Seal = ""
	if _, _, ok := Relocate(unsealed, GoFiles(root)); ok {
		t.Fatal("an unsealed anchor has nothing to compare and never moves")
	}
	if _, err := Reseal(root, a); err == nil {
		t.Fatal("reseal must refuse when the move cannot be proven")
	}
}

func TestRelocateSkipsFixturesDependenciesAndHiddenDirs(t *testing.T) {
	root := project(t)
	src := "package a\n\nfunc F() int { return 1 }\n"
	writeAll(t, root, "a.go", src)
	a, err := Make(root, "symbol:a.go::F")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "a.go")); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"testdata/a.go", "vendor/x/a.go", "node_modules/m/a.go", ".git/a.go", ".cache/a.go"} {
		writeAll(t, root, p, src)
	}
	if got, _, ok := Relocate(a, GoFiles(root)); ok {
		t.Fatalf("copies in fixtures, dependencies or hidden dirs are not the project's code: %+v", got)
	}
}

func TestRelocateStaysInsideTheProject(t *testing.T) {
	root := project(t)
	src := "package a\n\nfunc F() int { return 1 }\n"
	writeAll(t, root, "a.go", src)
	a, err := Make(root, "symbol:a.go::F")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "a.go")); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	writeAll(t, outside, "b.go", src)
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if got, _, ok := Relocate(a, GoFiles(root)); ok {
		t.Fatalf("a symlink out of the project must not be followed: %+v", got)
	}
	if err := os.Symlink(filepath.Join(outside, "b.go"), filepath.Join(root, "b.go")); err != nil {
		t.Fatal(err)
	}
	if got, _, ok := Relocate(a, GoFiles(root)); ok {
		t.Fatalf("a symlinked file out of the project must not be read: %+v", got)
	}
	for _, p := range GoFiles(root).paths {
		if strings.HasPrefix(p, "link/") || p == "b.go" {
			t.Fatalf("the scan must not include symlinks: %s", p)
		}
	}
}

func TestEnclosingDeclOfAContainsMatch(t *testing.T) {
	root := project(t)
	writeAll(t, root, "a.go", `package a

import "fmt"

// F prints.
func F() { fmt.Println("in F") }

type T struct{ n int }

func (t *T) M() int { return t.n + 41 }

var (
	A = "in group A"
	B = "in group B"
)

const C = "single const"

var x, y = "pair", 2

func G() {}
func H() {}
`)
	writeAll(t, root, "notes.md", "in F\n")
	for pattern, want := range map[string]string{
		`"in F"`: "F", `t\.n \+ 41`: "T.M", `"in group B"`: "B", `single const`: "C", `n int`: "T",
		`var \(`: "", `"pair"`: "", `F prints`: "", `func G\(\) \{\}\nfunc H`: "", `import "fmt"`: "",
	} {
		got, ok := EnclosingDecl(root, Anchor{Kind: KindContains, Path: "a.go", Arg: pattern})
		if got != want || ok != (want != "") {
			t.Errorf("EnclosingDecl(%q) = %q, %v; want %q", pattern, got, ok, want)
		}
	}
	if _, ok := EnclosingDecl(root, Anchor{Kind: KindContains, Path: "notes.md", Arg: "in F"}); ok {
		t.Error("only Go files have declarations")
	}
}

func TestCheckInOnlyAcceptsMatchesInsideTheRange(t *testing.T) {
	root := project(t)
	writeAll(t, root, "b.go", "package a\n\nfunc A() { _ = 30 }\n\nfunc B() { _ = 30 }\n")
	a := Anchor{Kind: KindContains, Path: "old.go", Arg: `_ = 30`}
	if r := CheckIn(root, a, "b.go", 5, 5); !r.OK || r.Line != 5 {
		t.Fatalf("the match inside lines 5-5 counts, even though an earlier one exists: %+v", r)
	}
	if r := CheckIn(root, a, "b.go", 1, 2); r.OK {
		t.Fatalf("no match inside lines 1-2: %+v", r)
	}
}
