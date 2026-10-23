// Package engine implements Aletheia's memory operations on top of the
// store: remembering with evidence, judging freshness, and recalling within
// a token budget.
package engine

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/firatmio/aletheia/internal/anchor"
	"github.com/firatmio/aletheia/internal/search"
	"github.com/firatmio/aletheia/internal/store"
)

// Limits and defaults.
const (
	MaxClaimRunes       = 400
	MaxBodyBytes        = 8000
	DefaultConfidence   = 0.8
	SimilarityThreshold = 0.6

	// UnanchoredTTL is the expiry given to memories without anchors when no
	// ttl is set: nothing else can ever mark them stale.
	UnanchoredTTL = 90 * 24 * time.Hour
)

// Freshness says how far a memory can be trusted right now.
type Freshness string

// Freshness values.
const (
	Verified     Freshness = "verified"     // every anchor holds against the files
	Unverifiable Freshness = "unverifiable" // no anchors: a hint, not a fact
	Stale        Freshness = "stale"        // an anchor failed or the memory expired
	// Moved: every anchor holds, but a sealed symbol's declaration was found
	// unchanged in another file. verify --reseal records the new place.
	Moved Freshness = "moved"
)

// Verdict is the result of judging one memory.
type Verdict struct {
	Freshness Freshness `json:"freshness"`
	Reasons   []string  `json:"reasons,omitempty"`
	Notes     []string  `json:"notes,omitempty"` // advice from Remember; Judge never sets it
	// Pointers say where the evidence is right now, e.g. "store.go:15-21",
	// so an agent can read those lines instead of the whole file.
	Pointers []string `json:"pointers,omitempty"`
}

// Engine runs memory operations against one store.
type Engine struct {
	Store *store.Store
	Now   func() time.Time
}

// New returns an Engine using the wall clock.
func New(s *store.Store) *Engine { return &Engine{Store: s, Now: time.Now} }

func (e *Engine) now() time.Time { return e.Now().UTC().Truncate(time.Second) }

// Judge checks a memory's anchors against the project right now. It reads
// files only; it never runs anything.
func (e *Engine) Judge(m *store.Memory) Verdict {
	return e.judge(m, &goScan{root: e.Store.Root})
}

// goScan lists the project's Go files at most once per operation, and only
// when a sealed symbol anchor needs relocating.
type goScan struct {
	root  string
	files *anchor.Files
}

func (s *goScan) list() *anchor.Files {
	if s.files == nil {
		s.files = anchor.GoFiles(s.root)
	}
	return s.files
}

// outcome is one anchor of a memory, checked against the project.
type outcome struct {
	a       anchor.Anchor
	bad     bool          // the anchor does not parse
	r       anchor.Result // the check against its own path
	to      *anchor.Moved // where its evidence is now, when it moved unchanged
	changed []string      // files declaring a symbol's name with other code
}

// assess checks every anchor of a memory. Sealed symbols whose declaration
// left its file are relocated; a contains anchor that fails then moves with
// a declaration of the same memory that moved from its file, but only if
// its pattern still matches inside that declaration. Whether a move keeps
// the claim true is the caller's to decide (see claimNames).
func (e *Engine) assess(m *store.Memory, scan *goScan) []outcome {
	out := make([]outcome, len(m.Anchors))
	moves := map[string][]anchor.Moved{} // old path -> declarations of this memory that moved from it
	for i, raw := range m.Anchors {
		a, err := anchor.Parse(raw)
		if err != nil {
			out[i] = outcome{bad: true}
			continue
		}
		o := outcome{a: a, r: anchor.Check(e.Store.Root, a)}
		if !o.r.OK && a.Kind == anchor.KindSymbol {
			to, changed, ok := anchor.Relocate(a, scan.list())
			o.changed = changed
			if ok {
				o.to = &to
				moves[a.Path] = append(moves[a.Path], to)
			}
		}
		out[i] = o
	}
	for i := range out {
		o := &out[i]
		if o.bad || o.r.OK || o.a.Kind != anchor.KindContains {
			continue
		}
		for _, d := range moves[o.a.Path] {
			if r := anchor.CheckIn(e.Store.Root, o.a, d.Path, d.Line, d.EndLine); r.OK {
				o.to = &anchor.Moved{Path: d.Path, Line: r.Line, EndLine: r.EndLine}
				break
			}
		}
	}
	return out
}

func (e *Engine) judge(m *store.Memory, scan *goScan) Verdict {
	var v Verdict
	var stale, bad, moved bool
	var movedReasons []string
	for _, o := range e.assess(m, scan) {
		switch {
		case o.bad:
			v.Reasons = append(v.Reasons, "bad anchor")
			bad = true
		case o.r.OK:
			if o.r.Line > 0 {
				v.Pointers = append(v.Pointers, pointer(o.a.Path, o.r.Line, o.r.EndLine))
			}
		// Code that moved unchanged keeps the claim true only if the claim
		// does not name the old file.
		case o.to != nil && !claimNames(m.Claim, o.a.Path):
			moved = true
			if r := o.a.Path + " → " + o.to.Path; !slices.Contains(movedReasons, r) {
				movedReasons = append(movedReasons, r)
			}
			if p := pointer(o.to.Path, o.to.Line, o.to.EndLine); !slices.Contains(v.Pointers, p) {
				v.Pointers = append(v.Pointers, p)
			}
		default:
			v.Reasons = append(v.Reasons, o.r.Detail)
			stale = true
		}
	}
	if m.Expires != nil && e.now().After(*m.Expires) {
		v.Reasons = append(v.Reasons, "expired")
		stale = true
	}
	switch {
	case stale:
		v.Freshness = Stale
	case bad || len(m.Anchors) == 0:
		v.Freshness = Unverifiable
	case moved:
		v.Freshness = Moved
		v.Reasons = movedReasons
	default:
		v.Freshness = Verified
	}
	return v
}

// claimNames reports whether a claim mentions a file by its path or name,
// e.g. "registered in builtin/builtin.go" or "in builtin.go".
func claimNames(claim, file string) bool {
	return strings.Contains(claim, file) || strings.Contains(claim, path.Base(file))
}

// claimPathNotes warns when the claim names the file of a symbol anchor. Such
// a memory goes stale instead of [moved] when the code moves (claimNames), so
// agents should describe the code's role and leave the path to the anchor.
func claimPathNotes(claim string, anchors []string) []string {
	var notes []string
	seen := map[string]bool{}
	for _, raw := range anchors {
		a, err := anchor.Parse(raw)
		if err != nil || a.Kind != anchor.KindSymbol || seen[a.Path] || !claimNames(claim, a.Path) {
			continue
		}
		seen[a.Path] = true
		notes = append(notes, "the claim names "+a.Path+", so the memory goes stale rather than [moved] if "+a.Arg+
			" moves; name the code's role, not its file, unless the file is the point")
	}
	return notes
}

func pointer(path string, start, end int) string {
	if end > start {
		return fmt.Sprintf("%s:%d-%d", path, start, end)
	}
	return fmt.Sprintf("%s:%d", path, start)
}

// pathToken matches words that look like project paths: "builtin/lib.go",
// "go.mod", "docs/". Identifiers such as fold.Visit match too, but only
// paths that exist become anchors.
var pathToken = regexp.MustCompile(`[A-Za-z0-9_.\-]+(?:[/\\][A-Za-z0-9_.\-]*)*`)

// maxNamedPaths caps how many anchors anchorNamedPaths adds to one memory.
const maxNamedPaths = 8

// anchorNamedPaths returns exists: anchors for files and directories named
// in the claim that exist and are not already covered by an anchor. A claim
// such as "register new builtins in builtin/builtin.go" then goes stale when
// that file is renamed, even if the agent anchored only other files. It
// also returns paths with a directory and an extension that do not exist:
// a claim naming them is probably wrong already.
func (e *Engine) anchorNamedPaths(claim string, covered map[string]bool) (added, missing []string) {
	for _, tok := range pathToken.FindAllString(claim, -1) {
		tok = strings.TrimRight(tok, ".-")
		if !strings.ContainsAny(tok, "./\\") {
			continue // a bare word
		}
		a, err := anchor.Make(e.Store.Root, "exists:"+tok)
		if err != nil {
			if strings.ContainsAny(tok, "/\\") && path.Ext(filepath.ToSlash(tok)) != "" && !strings.Contains(tok, "..") {
				missing = append(missing, tok)
			}
			continue
		}
		if covered[a.Path] || len(added) == maxNamedPaths {
			continue
		}
		covered[a.Path] = true
		added = append(added, a.String())
	}
	return added, missing
}

// symbolsForContains returns sealed symbol anchors for the Go declarations
// that contains anchors match inside, unless the memory already anchors
// them. Only a sealed symbol can be recognised after it moves (Relocate),
// and a contains anchor moves with the declaration it lies in (assess).
// Agents seldom write symbol anchors themselves; this gives their contains
// anchors the same protection.
func (e *Engine) symbolsForContains(anchors []string) []string {
	have := map[string]bool{}
	var contains []anchor.Anchor
	for _, raw := range anchors {
		a, err := anchor.Parse(raw)
		switch {
		case err != nil:
		case a.Kind == anchor.KindSymbol:
			have[a.Path+"::"+a.Arg] = true
		case a.Kind == anchor.KindContains:
			contains = append(contains, a)
		}
	}
	var out []string
	for _, a := range contains {
		if anchor.CommentOnly(e.Store.Root, a) {
			continue // a weak anchor already; a symbol would make it look stronger than it is
		}
		name, ok := anchor.EnclosingDecl(e.Store.Root, a)
		if !ok || have[a.Path+"::"+name] {
			continue
		}
		s, err := anchor.Make(e.Store.Root, "symbol:"+a.Path+"::"+name)
		if err != nil || s.Seal == "" {
			continue
		}
		have[a.Path+"::"+name] = true
		out = append(out, s.String())
	}
	return out
}

// RememberInput describes a new memory.
type RememberInput struct {
	Claim      string
	Body       string
	Source     string
	Tags       []string
	Anchors    []string // specs such as file:go.mod or contains:go.mod::go 1\.23
	Supersedes []string // ids (or unique prefixes) this memory replaces
	Confidence float64  // 0 means default
	TTL        time.Duration
	Force      bool // skip the duplicate/similarity check
}

// Similar is an existing memory close to a new claim.
type Similar struct {
	ID    string  `json:"id"`
	Claim string  `json:"claim"`
	Score float64 `json:"score"`
}

// ConflictError is returned when a new claim duplicates or closely matches
// an active memory. Nothing was written.
type ConflictError struct {
	Kind string // "duplicate" or "similar"
	With []Similar
}

func (c *ConflictError) Error() string {
	var parts []string
	for _, s := range c.With {
		parts = append(parts, fmt.Sprintf("%s %q (%.2f)", s.ID, s.Claim, s.Score))
	}
	return fmt.Sprintf("%s memory exists: %s; use supersedes=%s to replace it, or force to keep both",
		c.Kind, strings.Join(parts, ", "), c.With[0].ID)
}

func normalize(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

func cleanTags(in []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, raw := range in {
		for _, part := range strings.Split(raw, ",") {
			t := strings.ToLower(strings.TrimSpace(part))
			if t == "" || seen[t] {
				continue
			}
			if utf8.RuneCountInString(t) > 32 || strings.ContainsAny(t, "\r\n") {
				return nil, fmt.Errorf("invalid tag %q", t)
			}
			seen[t] = true
			out = append(out, t)
		}
	}
	if len(out) > 10 {
		return nil, errors.New("at most 10 tags")
	}
	return out, nil
}

func (e *Engine) findConflicts(claim string, skip map[string]bool) *ConflictError {
	norm := normalize(claim)
	toks := search.Tokenize(claim)
	all, _ := e.Store.List()
	var sims []Similar
	kind := "similar"
	for _, m := range all {
		if m.Status != store.StatusActive || skip[m.ID] {
			continue
		}
		if normalize(m.Claim) == norm {
			kind = "duplicate"
			sims = append(sims, Similar{ID: m.ID, Claim: m.Claim, Score: 1})
			continue
		}
		if j := search.Jaccard(toks, search.Tokenize(m.Claim)); j >= SimilarityThreshold {
			sims = append(sims, Similar{ID: m.ID, Claim: m.Claim, Score: j})
		}
	}
	if len(sims) == 0 {
		return nil
	}
	sort.SliceStable(sims, func(i, j int) bool { return sims[i].Score > sims[j].Score })
	if len(sims) > 3 {
		sims = sims[:3]
	}
	return &ConflictError{Kind: kind, With: sims}
}

// Remember stores a claim. File anchors are sealed with the current hash and
// every anchor must hold right now, so a memory can never start out
// contradicting its own evidence.
func (e *Engine) Remember(in RememberInput) (*store.Memory, Verdict, error) {
	claim := strings.Join(strings.Fields(in.Claim), " ")
	if n := utf8.RuneCountInString(claim); n < 3 || n > MaxClaimRunes {
		return nil, Verdict{}, fmt.Errorf("claim must be 3-%d characters: keep it to one short sentence", MaxClaimRunes)
	}
	if len(in.Body) > MaxBodyBytes {
		return nil, Verdict{}, fmt.Errorf("body too long (max %d bytes)", MaxBodyBytes)
	}
	tags, err := cleanTags(in.Tags)
	if err != nil {
		return nil, Verdict{}, err
	}
	conf := in.Confidence
	if conf == 0 {
		conf = DefaultConfidence
	}
	if conf < 0 || conf > 1 {
		return nil, Verdict{}, errors.New("confidence must be between 0 and 1")
	}
	source := strings.TrimSpace(in.Source)
	if len(source) > 64 {
		source = source[:64]
	}

	var anchors, notes []string
	covered := map[string]bool{}
	for _, spec := range in.Anchors {
		a, err := anchor.Make(e.Store.Root, spec)
		if err != nil {
			return nil, Verdict{}, fmt.Errorf("anchor %q: %w", spec, err)
		}
		if anchor.CommentOnly(e.Store.Root, a) {
			notes = append(notes, a.String()+" only matches comments, which can stay while the code changes; anchor the code itself")
		}
		anchors = append(anchors, a.String())
		covered[a.Path] = true
	}
	explicit := len(anchors)
	if s := e.symbolsForContains(anchors); len(s) > 0 {
		anchors = append(anchors, s...)
		notes = append(notes, "added "+strings.Join(s, ", ")+": the memory follows that code if it moves")
	}
	notes = append(notes, claimPathNotes(claim, anchors)...)
	added, missing := e.anchorNamedPaths(claim, covered)
	if len(added) > 0 {
		anchors = append(anchors, added...)
		notes = append(notes, "added anchors for files named in the claim: "+strings.Join(added, ", "))
	}
	if len(missing) > 0 {
		notes = append(notes, "the claim names files that do not exist: "+strings.Join(missing, ", "))
	}
	ttl := in.TTL
	if ttl == 0 && explicit == 0 { // exists: anchors on named files say little about the claim itself

		ttl = UnanchoredTTL
		notes = append(notes, fmt.Sprintf("no anchors: expires in %dd unless re-confirmed with verify --reseal",
			int(UnanchoredTTL.Hours()/24)))
	}

	var old []*store.Memory
	oldIDs := map[string]bool{}
	for _, ref := range in.Supersedes {
		m, err := e.Store.Get(ref)
		if err != nil {
			return nil, Verdict{}, err
		}
		if m.Status != store.StatusActive {
			return nil, Verdict{}, fmt.Errorf("memory %s is already superseded", m.ID)
		}
		if !oldIDs[m.ID] {
			oldIDs[m.ID] = true
			old = append(old, m)
		}
	}

	if !in.Force {
		if ce := e.findConflicts(claim, oldIDs); ce != nil {
			return nil, Verdict{}, ce
		}
	}

	id, err := e.Store.NewID()
	if err != nil {
		return nil, Verdict{}, err
	}
	now := e.now()
	m := &store.Memory{
		ID: id, Claim: claim, Body: strings.TrimSpace(in.Body), Tags: tags, Anchors: anchors,
		Confidence: conf, Status: store.StatusActive, Source: source, Created: now, Updated: now,
	}
	if ttl > 0 {
		exp := now.Add(ttl)
		m.Expires = &exp
	}
	for _, o := range old {
		m.Supersedes = append(m.Supersedes, o.ID)
	}
	if err := e.Store.Save(m); err != nil {
		return nil, Verdict{}, err
	}
	for _, o := range old {
		o.Status = store.StatusSuperseded
		o.SupersededBy = m.ID
		o.Updated = now
		if err := e.Store.Save(o); err != nil {
			return nil, Verdict{}, fmt.Errorf("saved %s but could not supersede %s: %w", m.ID, o.ID, err)
		}
	}
	v := e.Judge(m)
	v.Notes = notes
	return m, v, nil
}

// AnchorStatus is one anchor and whether it currently holds.
type AnchorStatus struct {
	Raw     string `json:"anchor"`
	OK      bool   `json:"ok"`
	Detail  string `json:"detail,omitempty"`
	MovedTo string `json:"moved_to,omitempty"` // "new.go:12-30" when a sealed symbol moved unchanged
}

// GetResult is a full memory with its current verdict.
type GetResult struct {
	Memory  *store.Memory
	Verdict Verdict
	Anchors []AnchorStatus
}

// Get returns a memory by id or unique prefix, with anchor-level detail.
func (e *Engine) Get(ref string) (*GetResult, error) {
	m, err := e.Store.Get(ref)
	if err != nil {
		return nil, err
	}
	scan := &goScan{root: e.Store.Root}
	g := &GetResult{Memory: m, Verdict: e.judge(m, scan)}
	for i, o := range e.assess(m, scan) {
		st := AnchorStatus{Raw: m.Anchors[i], OK: o.r.OK, Detail: o.r.Detail}
		switch {
		case o.bad:
			st.Detail = "bad anchor"
		case o.r.OK:
		case o.to != nil && !claimNames(m.Claim, o.a.Path):
			st.MovedTo = pointer(o.to.Path, o.to.Line, o.to.EndLine)
		case len(o.changed) > 0:
			// For a human deciding whether to reseal or supersede.
			st.Detail += "; " + o.a.Arg + " is declared, with other code, in " + strings.Join(o.changed, ", ")
		}
		g.Anchors = append(g.Anchors, st)
	}
	return g, nil
}

// VerifyItem is the freshness of one memory.
type VerifyItem struct {
	ID        string    `json:"id"`
	Claim     string    `json:"claim"`
	Freshness Freshness `json:"freshness"`
	Reasons   []string  `json:"reasons,omitempty"`
}

func item(m *store.Memory, v Verdict) VerifyItem {
	return VerifyItem{ID: m.ID, Claim: m.Claim, Freshness: v.Freshness, Reasons: v.Reasons}
}

// VerifyAll judges every active memory.
func (e *Engine) VerifyAll() []VerifyItem {
	all, _ := e.Store.List()
	var out []VerifyItem
	scan := &goScan{root: e.Store.Root}
	for _, m := range all {
		if m.Status == store.StatusActive {
			out = append(out, item(m, e.judge(m, scan)))
		}
	}
	return out
}

// Verify judges one memory.
func (e *Engine) Verify(ref string) (*VerifyItem, error) {
	m, err := e.Store.Get(ref)
	if err != nil {
		return nil, err
	}
	it := item(m, e.Judge(m))
	return &it, nil
}

// Reseal records that a stale memory was re-checked and still holds: file
// hashes and symbol fingerprints are refreshed, and a symbol that moved
// unchanged gets its new path. Pattern and existence anchors must already
// hold, so a claim that is actually wrong cannot be resealed; supersede it
// instead.
// A positive ttl sets a new expiry; unanchored memories get UnanchoredTTL
// when ttl is zero.
func (e *Engine) Reseal(ref string, ttl time.Duration) (*VerifyItem, error) {
	m, err := e.Store.Get(ref)
	if err != nil {
		return nil, err
	}
	if m.Status != store.StatusActive {
		return nil, fmt.Errorf("memory %s is superseded", m.ID)
	}
	sealed := make([]string, 0, len(m.Anchors))
	for i, o := range e.assess(m, &goScan{root: e.Store.Root}) {
		if o.bad {
			return nil, fmt.Errorf("anchor %q is not valid", m.Anchors[i])
		}
		a := o.a
		if !o.r.OK && o.to != nil {
			if claimNames(m.Claim, a.Path) {
				return nil, fmt.Errorf("the claim names %s, whose code moved to %s: supersede it instead", a.Path, o.to.Path)
			}
			a.Path = o.to.Path // the code moved unchanged: the anchor follows it
		}
		a2, err := anchor.Reseal(e.Store.Root, a)
		if err != nil {
			return nil, fmt.Errorf("claim no longer holds (%v): supersede it instead", err)
		}
		sealed = append(sealed, a2.String())
	}
	m.Anchors = sealed
	now := e.now()
	if ttl == 0 && len(m.Anchors) == 0 {
		ttl = UnanchoredTTL // re-confirming an unanchored memory buys it another period
	}
	if ttl > 0 {
		exp := now.Add(ttl)
		m.Expires = &exp
	}
	m.Updated = now
	if err := e.Store.Save(m); err != nil {
		return nil, err
	}
	it := item(m, e.Judge(m))
	return &it, nil
}
