// Package engine implements Aletheia's memory operations on top of the
// store: remembering with evidence, judging freshness, and recalling within
// a token budget.
package engine

import (
	"errors"
	"fmt"
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
)

// Verdict is the result of judging one memory.
type Verdict struct {
	Freshness Freshness `json:"freshness"`
	Reasons   []string  `json:"reasons,omitempty"`
	Notes     []string  `json:"notes,omitempty"` // advice from Remember; Judge never sets it
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
	v := Verdict{Freshness: Verified}
	if len(m.Anchors) == 0 {
		v.Freshness = Unverifiable
	}
	for _, raw := range m.Anchors {
		a, err := anchor.Parse(raw)
		if err != nil {
			v.Reasons = append(v.Reasons, "bad anchor")
			if v.Freshness == Verified {
				v.Freshness = Unverifiable
			}
			continue
		}
		if r := anchor.Check(e.Store.Root, a); !r.OK {
			v.Reasons = append(v.Reasons, r.Detail)
			v.Freshness = Stale
		}
	}
	if m.Expires != nil && e.now().After(*m.Expires) {
		v.Reasons = append(v.Reasons, "expired")
		v.Freshness = Stale
	}
	return v
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
	for _, spec := range in.Anchors {
		a, err := anchor.Make(e.Store.Root, spec)
		if err != nil {
			return nil, Verdict{}, fmt.Errorf("anchor %q: %w", spec, err)
		}
		if anchor.CommentOnly(e.Store.Root, a) {
			notes = append(notes, a.String()+" only matches comments, which can stay while the code changes; anchor the code itself")
		}
		anchors = append(anchors, a.String())
	}
	ttl := in.TTL
	if ttl == 0 && len(anchors) == 0 {
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
	Raw    string `json:"anchor"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
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
	g := &GetResult{Memory: m, Verdict: e.Judge(m)}
	for _, raw := range m.Anchors {
		st := AnchorStatus{Raw: raw}
		if a, err := anchor.Parse(raw); err != nil {
			st.Detail = "bad anchor"
		} else {
			r := anchor.Check(e.Store.Root, a)
			st.OK, st.Detail = r.OK, r.Detail
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
	for _, m := range all {
		if m.Status == store.StatusActive {
			out = append(out, item(m, e.Judge(m)))
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
// hashes are refreshed. Pattern and existence anchors must already hold, so
// a claim that is actually wrong cannot be resealed; supersede it instead.
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
	for _, raw := range m.Anchors {
		a, err := anchor.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("anchor %q: %w", raw, err)
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
