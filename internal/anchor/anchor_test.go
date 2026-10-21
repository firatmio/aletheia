package anchor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func project(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src", "auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func write(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFileAnchorDetectsChange(t *testing.T) {
	root := project(t)
	a, err := Make(root, "file:go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Arg) != 64 {
		t.Fatalf("expected sealed sha256, got %q", a.Arg)
	}
	if r := Check(root, a); !r.OK {
		t.Fatalf("fresh anchor should hold: %s", r.Detail)
	}
	write(t, root, "go.mod", "module x\n\ngo 1.24\n")
	if r := Check(root, a); r.OK || !strings.Contains(r.Detail, "changed") {
		t.Fatalf("expected 'changed', got %+v", r)
	}
	if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
		t.Fatal(err)
	}
	if r := Check(root, a); r.OK || !strings.Contains(r.Detail, "missing") {
		t.Fatalf("expected 'missing', got %+v", r)
	}
}

func TestContainsAnchor(t *testing.T) {
	root := project(t)
	a, err := Make(root, `contains:go.mod::^go 1\.23$`)
	if err != nil {
		t.Fatal(err)
	}
	if !Check(root, a).OK {
		t.Fatal("should hold")
	}
	if _, err := Make(root, `contains:go.mod::^go 1\.99$`); err == nil {
		t.Fatal("Make must refuse an anchor that does not hold right now")
	}
	write(t, root, "go.mod", "module x\n\ngo 1.24\n")
	if r := Check(root, a); r.OK || !strings.Contains(r.Detail, "pattern not found") {
		t.Fatalf("got %+v", r)
	}
}

func TestContainsSurvivesRealignment(t *testing.T) {
	root := project(t)
	write(t, root, "limits.go", "const (\n\tDefaultBudget = 600\n\tDefaultLimit  = 8\n)\n")
	a, err := Make(root, "contains:limits.go::DefaultLimit  = 8")
	if err != nil {
		t.Fatal(err)
	}
	// gofmt realigns the block after a longer name is added.
	write(t, root, "limits.go", "const (\n\tDefaultBudget       = 600\n\tDefaultLimit        = 8\n\tDefaultSessionLimit = 1000\n)\n")
	if r := Check(root, a); !r.OK {
		t.Fatalf("realignment must not stale the anchor: %s", r.Detail)
	}
	write(t, root, "limits.go", "const (\n\tDefaultLimit\t= 8\n)\n")
	if r := Check(root, a); !r.OK {
		t.Fatalf("tabs count as spaces: %s", r.Detail)
	}
	write(t, root, "limits.go", "const (\n\tDefaultLimit = 9\n)\n")
	if r := Check(root, a); r.OK {
		t.Fatal("a changed value must still stale the anchor")
	}
	write(t, root, "limits.go", "const (\n\tDefaultLimit=8\n)\n")
	if r := Check(root, a); r.OK {
		t.Fatal("spaces in the pattern still require at least one space")
	}
}

func TestRelaxSpacesKeepsRegexMeaning(t *testing.T) {
	for in, want := range map[string]string{
		"a  = 1":    `a[ \t]+=[ \t]+1`,
		"a +b":      "a +b",   // quantified space
		"a {2}b":    "a {2}b", // quantified space
		"x [ ]y":    `x[ \t]+[ ]y`,
		`a\ b`:      `a\ b`,
		"[^ ] z":    `[^ ][ \t]+z`,
		"[] ]{1} a": `[] ]{1}[ \t]+a`,
		"if x {":    `if[ \t]+x[ \t]+{`,
	} {
		if got := relaxSpaces(in); got != want {
			t.Errorf("relaxSpaces(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCommentOnly(t *testing.T) {
	root := project(t)
	write(t, root, "lethe.go", `// Package x. It never executes anything.
package x

type Options struct {
	Days int // archive after this many days (default 30)
}

/*
 * Retries: three by default.
 */
func defaults(o *Options) { o.Days = 30 }

var url = "http://example.com" // the "//" inside the string is not a comment
`)
	write(t, root, "notes.md", "# Never executes\n")
	for spec, want := range map[string]bool{
		"contains:lethe.go::never executes":    true,  // doc comment
		`contains:lethe.go::\(default 30\)`:    true,  // trailing comment
		"contains:lethe.go::Retries: three":    true,  // block comment
		`contains:lethe.go::o\.Days = 30`:      false, // code
		"contains:lethe.go::Days":              false, // matches code and comment
		"contains:lethe.go::example\\.com":     false, // string, not comment
		"contains:lethe.go::the \"//\" inside": true,
		"contains:notes.md::Never executes":    false, // unknown language
		"exists:lethe.go":                      false,
	} {
		a, err := Make(root, spec)
		if err != nil {
			t.Fatalf("%s: %v", spec, err)
		}
		if got := CommentOnly(root, a); got != want {
			t.Errorf("CommentOnly(%s) = %v, want %v", spec, got, want)
		}
	}
}

func TestExistsAnchor(t *testing.T) {
	root := project(t)
	a, err := Make(root, "exists:src/auth")
	if err != nil {
		t.Fatal(err)
	}
	if !Check(root, a).OK {
		t.Fatal("should hold")
	}
	if err := os.RemoveAll(filepath.Join(root, "src", "auth")); err != nil {
		t.Fatal(err)
	}
	if r := Check(root, a); r.OK || !strings.Contains(r.Detail, "missing") {
		t.Fatalf("got %+v", r)
	}
}

func TestParseRejectsEscapesAndGarbage(t *testing.T) {
	bad := []string{
		"file:../secret", "exists:/etc/passwd", "file:C:/Windows/win.ini", "file:a/../../b",
		"exists:.", "nonsense", "unknown:x", "contains:go.mod", "contains:go.mod::(unclosed",
		"file:", "",
	}
	for _, raw := range bad {
		if _, err := Parse(raw); err == nil {
			t.Errorf("Parse(%q) should fail", raw)
		}
	}
}

func TestParseStringRoundTrip(t *testing.T) {
	hash := strings.Repeat("0f", 32)
	for _, raw := range []string{
		"file:go.mod@" + hash, `contains:src/a.go::func Main\(`, "exists:src/auth", "file:go.mod",
	} {
		a, err := Parse(raw)
		if err != nil {
			t.Fatalf("Parse(%q): %v", raw, err)
		}
		if a.String() != raw {
			t.Errorf("round trip: %q -> %q", raw, a.String())
		}
	}
}

func TestLineEndingsDoNotMatter(t *testing.T) {
	// The same commit checked out with core.autocrlf on Windows must verify.
	root := project(t)
	a, err := Make(root, "file:go.mod")
	if err != nil {
		t.Fatal(err)
	}
	c, err := Make(root, `contains:go.mod::^go 1\.23$`)
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "go.mod", "module x\r\n\r\ngo 1.23\r\n")
	if r := Check(root, a); !r.OK {
		t.Fatalf("CRLF checkout should still verify: %s", r.Detail)
	}
	if r := Check(root, c); !r.OK {
		t.Fatalf("anchored regex should match across CRLF: %s", r.Detail)
	}
	// Binary files are hashed byte for byte.
	write(t, root, "blob.bin", "a\x00b\r\n")
	b, err := Make(root, "file:blob.bin")
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "blob.bin", "a\x00b\n")
	if Check(root, b).OK {
		t.Fatal("binary content change must be detected")
	}
}

func TestUnsealedFileAnchorDoesNotVerify(t *testing.T) {
	root := project(t)
	a, err := Parse("file:go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if r := Check(root, a); r.OK {
		t.Fatal("an unsealed anchor must not count as verified")
	}
}

func TestSymlinkCannotEscapeRoot(t *testing.T) {
	root := project(t)
	outside := t.TempDir()
	write(t, outside, "secret.txt", "top secret")
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	a := Anchor{Kind: KindContains, Path: "link/secret.txt", Arg: "secret"}
	if r := Check(root, a); r.OK || !strings.Contains(r.Detail, "outside") {
		t.Fatalf("symlink escape must be refused, got %+v", r)
	}
}

func TestReseal(t *testing.T) {
	root := project(t)
	a, err := Make(root, "file:go.mod")
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "go.mod", "module x\n\ngo 1.24\n")
	if Check(root, a).OK {
		t.Fatal("should be stale")
	}
	b, err := Reseal(root, a)
	if err != nil {
		t.Fatal(err)
	}
	if b.Arg == a.Arg || !Check(root, b).OK {
		t.Fatal("reseal should refresh the hash")
	}
	c := Anchor{Kind: KindContains, Path: "go.mod", Arg: `go 1\.23`}
	if _, err := Reseal(root, c); err == nil {
		t.Fatal("reseal must not paper over a pattern that no longer holds")
	}
}
