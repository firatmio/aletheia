package store

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMemoryRoundTrip(t *testing.T) {
	now := time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC)
	exp := now.Add(48 * time.Hour)
	want := &Memory{
		ID:           "a3f9c2",
		Claim:        `Uses "quotes": colons, <html> & ünïcode 🙂`,
		Body:         "line one\n---\nline three",
		Tags:         []string{"backend", "go"},
		Anchors:      []string{"file:go.mod@" + strings.Repeat("ab", 32), `contains:go.mod::go 1\.23`},
		Confidence:   0.9,
		Status:       StatusActive,
		Source:       "claude-code",
		Created:      now,
		Updated:      now,
		Expires:      &exp,
		Supersedes:   []string{"b1c2d3"},
		SupersededBy: "",
	}
	got, err := ParseMemory("a3f9c2", want.Marshal())
	if err != nil {
		t.Fatal(err)
	}
	if got.Claim != want.Claim || got.Body != want.Body || got.Confidence != want.Confidence ||
		got.Source != want.Source || got.Status != want.Status {
		t.Fatalf("scalar mismatch:\n got %+v\nwant %+v", got, want)
	}
	if !reflect.DeepEqual(got.Tags, want.Tags) || !reflect.DeepEqual(got.Anchors, want.Anchors) ||
		!reflect.DeepEqual(got.Supersedes, want.Supersedes) {
		t.Fatalf("list mismatch:\n got %+v\nwant %+v", got, want)
	}
	if !got.Created.Equal(now) || !got.Updated.Equal(now) || got.Expires == nil || !got.Expires.Equal(exp) {
		t.Fatalf("time mismatch: %+v", got)
	}
}

func TestMarshalDoesNotHTMLEscape(t *testing.T) {
	m := &Memory{ID: "abc123", Claim: "a <b> & c", Confidence: 0.8}
	if out := string(m.Marshal()); !strings.Contains(out, `claim: "a <b> & c"`) {
		t.Fatalf("claim must be written verbatim, not HTML-escaped:\n%s", out)
	}
}

func TestParseHandWritten(t *testing.T) {
	src := "---\r\nclaim: Uses Go 1.23\r\ntags: [backend, \"go\"]\r\nconfidence: 0.5\r\n# a comment\r\n---\r\nSome body\r\n"
	m, err := ParseMemory("abc123", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if m.Claim != "Uses Go 1.23" || m.Confidence != 0.5 || m.Body != "Some body" {
		t.Fatalf("unexpected: %+v", m)
	}
	if !reflect.DeepEqual(m.Tags, []string{"backend", "go"}) {
		t.Fatalf("tags: %v", m.Tags)
	}
}

func TestUnknownKeysPreserved(t *testing.T) {
	src := "---\nclaim: \"x y z\"\nfuture_key: \"v\"\nfuture_list:\n  - \"a\"\n  - \"b\"\n---\nbody\n"
	m, err := ParseMemory("abc123", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out := string(m.Marshal())
	for _, want := range []string{`future_key: "v"`, "future_list:", `  - "a"`, `  - "b"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("lost %q in:\n%s", want, out)
		}
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"no frontmatter": "just text",
		"unterminated":   "---\nclaim: x y z\n",
		"missing claim":  "---\nconfidence: 0.5\n---\nbody\n",
		"bad line":       "---\nthis is not a key value\n---\n",
		"bad confidence": "---\nclaim: x y z\nconfidence: lots\n---\n",
	}
	for name, src := range cases {
		if _, err := ParseMemory("abc123", []byte(src)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func newStore(t *testing.T) *Store {
	t.Helper()
	s, _, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func save(t *testing.T, s *Store, id, claim string) {
	t.Helper()
	if err := s.Save(&Memory{ID: id, Claim: claim, Confidence: 0.8, Status: StatusActive}); err != nil {
		t.Fatal(err)
	}
}

func TestInitIsIdempotent(t *testing.T) {
	root := t.TempDir()
	if _, created, err := Init(root); err != nil || !created {
		t.Fatalf("first init: created=%v err=%v", created, err)
	}
	if _, created, err := Init(root); err != nil || created {
		t.Fatalf("second init: created=%v err=%v", created, err)
	}
	gi, err := os.ReadFile(filepath.Join(root, DirName, ".gitignore"))
	if err != nil || !strings.Contains(string(gi), ".state/") {
		t.Fatalf("gitignore: %q %v", gi, err)
	}
}

func TestGetPrefixAndAmbiguity(t *testing.T) {
	s := newStore(t)
	save(t, s, "aaa111", "claim one")
	save(t, s, "aab222", "claim two")
	save(t, s, "bbb333", "claim three")

	if m, err := s.Get("bbb"); err != nil || m.ID != "bbb333" {
		t.Fatalf("prefix get: %v %v", m, err)
	}
	if _, err := s.Get("aa"); !errors.Is(err, ErrAmbiguous) {
		t.Fatalf("want ambiguous, got %v", err)
	}
	for _, bad := range []string{"../x", "zzz", "", "A B"} {
		if _, err := s.Get(bad); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Get(%q): want not found, got %v", bad, err)
		}
	}
}

func TestListReportsBadFilesWithoutHidingGoodOnes(t *testing.T) {
	s := newStore(t)
	save(t, s, "good01", "a fine memory")
	if err := os.WriteFile(filepath.Join(s.memDir(), "broken.md"), []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, problems := s.List()
	if len(got) != 1 || got[0].ID != "good01" || len(problems) != 1 {
		t.Fatalf("got %v problems %v", got, problems)
	}
}

func TestOpenWalksUp(t *testing.T) {
	root := t.TempDir()
	if _, _, err := Init(root); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := Open(sub)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(root)
	got, _ := filepath.EvalSymlinks(s.Root)
	if got != want {
		t.Fatalf("root %q, want %q", got, want)
	}
	if _, err := Open(t.TempDir()); !errors.Is(err, ErrNoStore) {
		t.Fatalf("want ErrNoStore, got %v", err)
	}
}

func TestArchiveMovesInsteadOfDeleting(t *testing.T) {
	s := newStore(t)
	save(t, s, "old001", "an old memory")
	if err := s.Archive("old001"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("old001"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("still active: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.archiveDir(), "old001.md")); err != nil {
		t.Fatalf("not archived: %v", err)
	}
}

func TestSaveRejectsUnsafeIDs(t *testing.T) {
	s := newStore(t)
	for _, id := range []string{"../evil", "a/b", "", "x"} {
		if err := s.Save(&Memory{ID: id, Claim: "c c c"}); err == nil {
			t.Errorf("Save accepted id %q", id)
		}
	}
}

func TestNewIDIsValidAndUnique(t *testing.T) {
	s := newStore(t)
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		id, err := s.NewID()
		if err != nil || !ValidID(id) || len(id) != 6 || seen[id] {
			t.Fatalf("bad id %q err=%v", id, err)
		}
		seen[id] = true
		save(t, s, id, "claim "+id)
	}
}
