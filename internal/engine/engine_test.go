package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/firatmio/aletheia/internal/store"
)

type env struct {
	e    *Engine
	root string
	now  time.Time
}

func (v *env) advance(d time.Duration) { v.now = v.now.Add(d) }

func (v *env) write(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(v.root, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newEnv(t *testing.T) *env {
	t.Helper()
	root := t.TempDir()
	s, _, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	v := &env{root: root, now: time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)}
	v.e = New(s)
	v.e.Now = func() time.Time { return v.now }
	v.write(t, "go.mod", "module x\n\ngo 1.23\n")
	return v
}

func (v *env) remember(t *testing.T, in RememberInput) *store.Memory {
	t.Helper()
	m, _, err := v.e.Remember(in)
	if err != nil {
		t.Fatalf("Remember(%q): %v", in.Claim, err)
	}
	return m
}

func TestVerifiedThenStaleThenSuperseded(t *testing.T) {
	v := newEnv(t)
	m, verdict, err := v.e.Remember(RememberInput{
		Claim:   "The project targets Go 1.23",
		Anchors: []string{"file:go.mod", `contains:go.mod::^go 1\.23$`},
	})
	if err != nil || verdict.Freshness != Verified {
		t.Fatalf("remember: %v %+v", err, verdict)
	}

	res, err := v.e.Recall(RecallOptions{Query: "go version"})
	if err != nil || len(res.Hits) != 1 || res.Hits[0].Freshness != Verified {
		t.Fatalf("recall before change: %v %+v", err, res)
	}

	v.write(t, "go.mod", "module x\n\ngo 1.24\n")

	res, _ = v.e.Recall(RecallOptions{Query: "go version"})
	if len(res.Hits) != 0 || len(res.HiddenStale) != 1 || res.HiddenStale[0] != m.ID {
		t.Fatalf("stale memory must be hidden: %+v", res)
	}
	if !strings.Contains(res.Text(), "stale hidden") {
		t.Fatalf("footer should mention hidden stale memories: %q", res.Text())
	}
	res, _ = v.e.Recall(RecallOptions{Query: "go version", IncludeStale: true})
	if len(res.Hits) != 1 || res.Hits[0].Freshness != Stale || !strings.Contains(res.Text(), "go.mod: changed") {
		t.Fatalf("include-stale should show the reason: %q", res.Text())
	}

	if _, err := v.e.Reseal(m.ID, 0); err == nil {
		t.Fatal("reseal must fail: the claim is no longer true")
	}

	nm, _, err := v.e.Remember(RememberInput{
		Claim:      "The project targets Go 1.24",
		Anchors:    []string{"file:go.mod", `contains:go.mod::^go 1\.24$`},
		Supersedes: []string{m.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	old, _ := v.e.Store.Get(m.ID)
	if old.Status != store.StatusSuperseded || old.SupersededBy != nm.ID {
		t.Fatalf("old memory should be superseded: %+v", old)
	}
	res, _ = v.e.Recall(RecallOptions{Query: "go version"})
	if len(res.Hits) != 1 || res.Hits[0].ID != nm.ID || len(res.HiddenStale) != 0 {
		t.Fatalf("only the new memory should be recalled: %+v", res)
	}
}

func TestRememberRefusesAnchorThatDoesNotHold(t *testing.T) {
	v := newEnv(t)
	_, _, err := v.e.Remember(RememberInput{
		Claim: "The project targets Go 1.99", Anchors: []string{`contains:go.mod::go 1\.99`},
	})
	if err == nil {
		t.Fatal("a memory must not start out contradicting its own evidence")
	}
}

func TestResealAfterHarmlessChange(t *testing.T) {
	v := newEnv(t)
	m := v.remember(t, RememberInput{Claim: "go.mod exists and names the module x", Anchors: []string{"file:go.mod", `contains:go.mod::module x`}})
	v.write(t, "go.mod", "module x\n\ngo 1.23\n\n// harmless comment\n")
	if got, _ := v.e.Verify(m.ID); got.Freshness != Stale {
		t.Fatalf("expected stale after edit, got %v", got.Freshness)
	}
	it, err := v.e.Reseal(m.ID, 0)
	if err != nil || it.Freshness != Verified {
		t.Fatalf("reseal: %v %+v", err, it)
	}
}

func TestConflictDetection(t *testing.T) {
	v := newEnv(t)
	first := v.remember(t, RememberInput{Claim: "The backend uses Go 1.23 and PostgreSQL"})

	_, _, err := v.e.Remember(RememberInput{Claim: "The backend uses Go 1.23 and PostgreSQL 16"})
	var ce *ConflictError
	if !errors.As(err, &ce) || ce.Kind != "similar" || ce.With[0].ID != first.ID {
		t.Fatalf("want similar conflict with %s, got %v", first.ID, err)
	}

	_, _, err = v.e.Remember(RememberInput{Claim: "  the backend uses GO 1.23   and postgresql "})
	if !errors.As(err, &ce) || ce.Kind != "duplicate" {
		t.Fatalf("want duplicate, got %v", err)
	}

	if _, _, err := v.e.Remember(RememberInput{Claim: "The backend uses Go 1.23 and PostgreSQL 16", Force: true}); err != nil {
		t.Fatalf("force should skip the check: %v", err)
	}
}

func TestRecallRespectsBudget(t *testing.T) {
	v := newEnv(t)
	for i := 0; i < 10; i++ {
		v.remember(t, RememberInput{
			Claim: fmt.Sprintf("component %d handles %s processing for tenants", i, strings.Repeat("very long ", 5)),
			Force: true,
		})
	}
	res, err := v.e.Recall(RecallOptions{Query: "component", Budget: 60, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Hits) == 0 || len(res.Hits) >= 10 || res.Omitted == 0 {
		t.Fatalf("budget should truncate: hits=%d omitted=%d tokens=%d", len(res.Hits), res.Omitted, res.Tokens)
	}
	if res.Tokens > 60 {
		t.Fatalf("used %d tokens, budget 60", res.Tokens)
	}
	if !strings.Contains(res.Text(), "more)") {
		t.Fatalf("footer should report omitted memories: %q", res.Text())
	}
}

func TestRecallAlwaysReturnsAtLeastOneHit(t *testing.T) {
	v := newEnv(t)
	v.remember(t, RememberInput{Claim: strings.Repeat("enormous ", 40)})
	res, _ := v.e.Recall(RecallOptions{Query: "enormous", Budget: 5})
	if len(res.Hits) != 1 {
		t.Fatalf("want the single best hit even over budget, got %d", len(res.Hits))
	}
}

func TestSessionDelta(t *testing.T) {
	v := newEnv(t)
	v.remember(t, RememberInput{Claim: "Deploys go through GitHub Actions"})
	a, err := v.e.Recall(RecallOptions{Query: "deploys", Session: "s1"})
	if err != nil || len(a.Hits) != 1 {
		t.Fatalf("first recall: %v %+v", err, a)
	}
	b, _ := v.e.Recall(RecallOptions{Query: "deploys", Session: "s1"})
	if len(b.Hits) != 0 || b.AlreadySent != 1 || !strings.Contains(b.Text(), "already sent") {
		t.Fatalf("second recall should skip: %+v", b)
	}
	c, _ := v.e.Recall(RecallOptions{Query: "deploys", Session: "other"})
	if len(c.Hits) != 1 {
		t.Fatalf("a different session starts fresh: %+v", c)
	}
	if _, err := v.e.Recall(RecallOptions{Query: "x y", Session: "../escape"}); err == nil {
		t.Fatal("session names must not be able to traverse paths")
	}
}

func TestExpiry(t *testing.T) {
	v := newEnv(t)
	m := v.remember(t, RememberInput{Claim: "Staging DB is being migrated this week", Tags: []string{"temp"}, TTL: 24 * time.Hour})
	if got, _ := v.e.Verify(m.ID); got.Freshness != Unverifiable {
		t.Fatalf("unanchored memory is unverifiable, got %v", got.Freshness)
	}
	v.advance(48 * time.Hour)
	got, _ := v.e.Verify(m.ID)
	if got.Freshness != Stale || got.Reasons[0] != "expired" {
		t.Fatalf("want expired, got %+v", got)
	}
	got, err := v.e.Reseal(m.ID, 7*24*time.Hour)
	if err != nil || got.Freshness != Unverifiable {
		t.Fatalf("reseal with ttl should renew: %v %+v", err, got)
	}
}

func TestUnanchoredMemoryExpiresByDefault(t *testing.T) {
	v := newEnv(t)
	m, verdict, err := v.e.Remember(RememberInput{Claim: "The API has no authentication"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Expires == nil || !m.Expires.Equal(v.now.Add(UnanchoredTTL)) {
		t.Fatalf("unanchored memory should expire after %v, got %v", UnanchoredTTL, m.Expires)
	}
	if len(verdict.Notes) != 1 || !strings.Contains(verdict.Notes[0], "expires in 90d") {
		t.Fatalf("the agent should be told it expires: %q", verdict.Notes)
	}
	if s := RenderSaved(m, verdict); !strings.Contains(s, "\nnote: no anchors") {
		t.Fatalf("note missing from output: %q", s)
	}

	v.advance(UnanchoredTTL + time.Hour)
	if got, _ := v.e.Verify(m.ID); got.Freshness != Stale {
		t.Fatalf("want expired, got %+v", got)
	}
	if got, err := v.e.Reseal(m.ID, 0); err != nil || got.Freshness != Unverifiable {
		t.Fatalf("reseal without ttl should renew an unanchored memory: %v %+v", err, got)
	}

	explicit := v.remember(t, RememberInput{Claim: "Staging is frozen until Friday", TTL: 72 * time.Hour})
	if !explicit.Expires.Equal(v.now.Add(72 * time.Hour)) {
		t.Fatalf("an explicit ttl wins: %v", explicit.Expires)
	}
	anchored := v.remember(t, RememberInput{Claim: "Go version lives in go.mod", Anchors: []string{"exists:go.mod"}})
	if anchored.Expires != nil {
		t.Fatalf("anchored memories do not expire by default: %v", anchored.Expires)
	}
}

func TestRememberWarnsAboutCommentOnlyAnchors(t *testing.T) {
	v := newEnv(t)
	v.write(t, "lethe.go", "package x\n\ntype Options struct {\n\tDays int // default 30\n}\n\nfunc defaults(o *Options) { o.Days = 30 }\n")
	_, verdict, err := v.e.Remember(RememberInput{
		Claim: "lethe archives after 30 days by default", Anchors: []string{"contains:lethe.go::default 30"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if verdict.Freshness != Verified || len(verdict.Notes) != 1 || !strings.Contains(verdict.Notes[0], "only matches comments") {
		t.Fatalf("want a saved memory with a warning, got %+v", verdict)
	}
	_, verdict, err = v.e.Remember(RememberInput{
		Claim: "Default archive age is set in defaults()", Anchors: []string{`contains:lethe.go::o\.Days = 30`},
	})
	if err != nil || len(verdict.Notes) != 0 {
		t.Fatalf("a code anchor needs no warning: %v %+v", err, verdict)
	}
}

func TestUnanchoredRanksBelowVerified(t *testing.T) {
	v := newEnv(t)
	v.remember(t, RememberInput{Claim: "Linting runs in CI hint", Force: true})
	ver := v.remember(t, RememberInput{Claim: "Linting configuration lives in go.mod comments", Anchors: []string{"exists:go.mod"}, Force: true})
	res, _ := v.e.Recall(RecallOptions{Query: "linting"})
	if len(res.Hits) != 2 || res.Hits[0].ID != ver.ID {
		t.Fatalf("verified memory should rank first: %+v", res.Hits)
	}
}

func TestTagFilterAndEmptyQuery(t *testing.T) {
	v := newEnv(t)
	v.remember(t, RememberInput{Claim: "Frontend uses React", Tags: []string{"frontend"}})
	v.advance(time.Minute)
	v.remember(t, RememberInput{Claim: "Backend written in Go", Tags: []string{"backend"}})
	res, _ := v.e.Recall(RecallOptions{Tag: "frontend"})
	if len(res.Hits) != 1 || res.Hits[0].Claim != "Frontend uses React" {
		t.Fatalf("tag filter: %+v", res.Hits)
	}
	res, _ = v.e.Recall(RecallOptions{})
	if len(res.Hits) != 2 || res.Hits[0].Claim != "Backend written in Go" {
		t.Fatalf("empty query should list newest first: %+v", res.Hits)
	}
	res, _ = v.e.Recall(RecallOptions{Query: "?!"})
	if len(res.Hits) != 0 || res.Text() != "no matching memories" {
		t.Fatalf("unsearchable query: %q", res.Text())
	}
}

func TestLethe(t *testing.T) {
	v := newEnv(t)
	old := v.remember(t, RememberInput{Claim: "Tabs are used for indentation"})
	v.remember(t, RememberInput{Claim: "Spaces are used for indentation", Supersedes: []string{old.ID}, Force: true})
	stale := v.remember(t, RememberInput{Claim: "Module is declared as x", Anchors: []string{"contains:go.mod::module x"}})
	v.write(t, "go.mod", "module y\n")
	v.advance(40 * 24 * time.Hour)

	dry, err := v.e.Lethe(LetheOptions{Days: 30})
	if err != nil || len(dry.Items) != 2 || dry.Applied {
		t.Fatalf("dry run: %v %+v", err, dry)
	}
	if _, err := v.e.Store.Get(old.ID); err != nil {
		t.Fatalf("dry run must not move anything: %v", err)
	}
	done, err := v.e.Lethe(LetheOptions{Days: 30, Apply: true})
	if err != nil || done.Archived != 2 {
		t.Fatalf("apply: %v %+v", err, done)
	}
	for _, id := range []string{old.ID, stale.ID} {
		if _, err := v.e.Store.Get(id); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("%s should be archived: %v", id, err)
		}
		if _, err := os.Stat(filepath.Join(v.e.Store.Dir, "archive", id+".md")); err != nil {
			t.Fatalf("archived file missing: %v", err)
		}
	}
}

func TestStats(t *testing.T) {
	v := newEnv(t)
	v.remember(t, RememberInput{Claim: "Module file exists at the root", Anchors: []string{"exists:go.mod"}, Body: strings.Repeat("detail ", 100)})
	v.remember(t, RememberInput{Claim: "Team prefers short pull requests"})
	st := v.e.Stats()
	if st.Total != 2 || st.Active != 2 || st.Verified != 1 || st.Unverifiable != 1 || st.Stale != 0 {
		t.Fatalf("unexpected: %+v", st)
	}
	if st.IndexTokens <= 0 || st.IndexTokens >= st.FullTokens {
		t.Fatalf("compact index should be cheaper than loading everything: %+v", st)
	}
}

func TestInputValidation(t *testing.T) {
	v := newEnv(t)
	bad := []RememberInput{
		{Claim: "no"},
		{Claim: strings.Repeat("x", MaxClaimRunes+1)},
		{Claim: "valid claim here", Body: strings.Repeat("b", MaxBodyBytes+1)},
		{Claim: "valid claim here", Confidence: 1.5},
		{Claim: "valid claim here", Supersedes: []string{"nope00"}},
		{Claim: "valid claim here", Anchors: []string{"file:../outside"}},
		{Claim: "valid claim here", Tags: []string{strings.Repeat("t", 40)}},
	}
	for i, in := range bad {
		if _, _, err := v.e.Remember(in); err == nil {
			t.Errorf("case %d should have been rejected: %+v", i, in)
		}
	}
}

func TestCannotSupersedeTwice(t *testing.T) {
	v := newEnv(t)
	old := v.remember(t, RememberInput{Claim: "Port is 3000"})
	v.remember(t, RememberInput{Claim: "Port is 8080", Supersedes: []string{old.ID}})
	if _, _, err := v.e.Remember(RememberInput{Claim: "Port is 9090", Supersedes: []string{old.ID}}); err == nil {
		t.Fatal("superseding an already superseded memory must fail")
	}
}

func TestParseTTL(t *testing.T) {
	ok := map[string]time.Duration{
		"": 0, "30d": 30 * 24 * time.Hour, "2w": 14 * 24 * time.Hour, "12h": 12 * time.Hour, "90m": 90 * time.Minute,
	}
	for in, want := range ok {
		if got, err := ParseTTL(in); err != nil || got != want {
			t.Errorf("ParseTTL(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"x", "-5d", "0d", "d", "5y"} {
		if _, err := ParseTTL(in); err == nil {
			t.Errorf("ParseTTL(%q) should fail", in)
		}
	}
}
