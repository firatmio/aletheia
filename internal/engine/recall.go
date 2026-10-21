package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/firatmio/aletheia/internal/search"
	"github.com/firatmio/aletheia/internal/store"
)

// Recall defaults.
const (
	DefaultBudget = 600 // estimated tokens
	DefaultLimit  = 8
)

// RecallOptions controls a recall.
type RecallOptions struct {
	Query        string
	Budget       int // estimated-token budget for the rendered answer
	Limit        int // maximum number of memories
	Full         bool
	IncludeStale bool
	Session      string // when set, memories already sent in this session are skipped
	Tag          string
}

// Hit is one recalled memory.
type Hit struct {
	ID         string    `json:"id"`
	Claim      string    `json:"claim"`
	Body       string    `json:"body,omitempty"`
	Freshness  Freshness `json:"freshness"`
	Reasons    []string  `json:"reasons,omitempty"`
	Pointers   []string  `json:"pointers,omitempty"`
	Confidence float64   `json:"confidence"`
	Score      float64   `json:"score"`
	Tokens     int       `json:"tokens"`
	line       string
}

// RecallResult is the packed answer to a recall.
type RecallResult struct {
	Hits        []Hit    `json:"hits"`
	HiddenStale []string `json:"hidden_stale,omitempty"`
	AlreadySent int      `json:"already_sent,omitempty"`
	Omitted     int      `json:"omitted,omitempty"`
	Tokens      int      `json:"tokens"`
}

func hasTag(m *store.Memory, tag string) bool {
	tag = strings.ToLower(strings.TrimSpace(tag))
	for _, t := range m.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

func docTerms(m *store.Memory) []string {
	var t []string
	ct := search.Tokenize(m.Claim)
	for i := 0; i < 3; i++ {
		t = append(t, ct...)
	}
	tt := search.Tokenize(strings.Join(m.Tags, " "))
	t = append(t, tt...)
	t = append(t, tt...)
	return append(t, search.Tokenize(m.Body)...)
}

var freshnessFactor = map[Freshness]float64{Verified: 1, Unverifiable: 0.7, Stale: 0.3}

// Recall ranks memories by relevance, verifies the best candidates against
// the project, hides stale ones, and packs the answer into the token budget.
func (e *Engine) Recall(o RecallOptions) (*RecallResult, error) {
	if o.Budget <= 0 {
		o.Budget = DefaultBudget
	}
	if o.Limit <= 0 {
		o.Limit = DefaultLimit
	}
	if o.Session != "" {
		if _, err := e.sessionPath(o.Session); err != nil {
			return nil, err
		}
	}
	all, _ := e.Store.List()
	var pool []*store.Memory
	for _, m := range all {
		if m.Status != store.StatusActive {
			continue
		}
		if o.Tag != "" && !hasTag(m, o.Tag) {
			continue
		}
		pool = append(pool, m)
	}

	res := &RecallResult{Hits: []Hit{}}
	type cand struct {
		m   *store.Memory
		rel float64
	}
	var cands []cand
	query := strings.TrimSpace(o.Query)
	qt := search.Tokenize(query)
	switch {
	case len(qt) > 0:
		docs := make([]search.Doc, len(pool))
		for i, m := range pool {
			docs[i] = search.Doc{ID: m.ID, Terms: docTerms(m)}
		}
		scores := search.NewIndex(docs).Scores(qt)
		var max float64
		for _, s := range scores {
			if s > max {
				max = s
			}
		}
		for _, m := range pool {
			if s := scores[m.ID]; s > 0 {
				cands = append(cands, cand{m, s / max})
			}
		}
	case query != "":
		return res, nil // the query had no searchable words
	default:
		for _, m := range pool {
			cands = append(cands, cand{m, 1})
		}
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].rel != cands[j].rel {
			return cands[i].rel > cands[j].rel
		}
		return cands[i].m.Updated.After(cands[j].m.Updated)
	})
	if k := min(o.Limit*4, 40); len(cands) > k {
		cands = cands[:k]
	}

	sent := map[string]bool{}
	if o.Session != "" {
		s, err := e.loadSession(o.Session)
		if err != nil {
			return nil, err
		}
		sent = s
	}

	var ranked []Hit
	for _, c := range cands {
		if sent[c.m.ID] {
			res.AlreadySent++
			continue
		}
		v := e.Judge(c.m)
		if v.Freshness == Stale && !o.IncludeStale {
			res.HiddenStale = append(res.HiddenStale, c.m.ID)
			continue
		}
		h := Hit{
			ID: c.m.ID, Claim: c.m.Claim, Freshness: v.Freshness, Reasons: v.Reasons, Pointers: v.Pointers,
			Confidence: c.m.Confidence, Score: c.rel * c.m.Confidence * freshnessFactor[v.Freshness],
		}
		if o.Full {
			h.Body = c.m.Body
		}
		ranked = append(ranked, h)
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].Score > ranked[j].Score })

	used := 0
	for _, h := range ranked {
		line := renderHit(h)
		cost := search.EstimateTokens(line) + 1
		if h.Body != "" && used+cost > o.Budget {
			h.Body = ""
			line = renderHit(h)
			cost = search.EstimateTokens(line) + 1
		}
		if len(res.Hits) >= o.Limit || (len(res.Hits) > 0 && used+cost > o.Budget) {
			res.Omitted++
			continue
		}
		h.Tokens, h.line = cost, line
		used += cost
		res.Hits = append(res.Hits, h)
	}
	res.Tokens = used

	if o.Session != "" && len(res.Hits) > 0 {
		for _, h := range res.Hits {
			sent[h.ID] = true
		}
		if err := e.saveSession(o.Session, sent); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// --- per-session "already sent" tracking -----------------------------------

var sessionPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func (e *Engine) sessionPath(name string) (string, error) {
	if !sessionPattern.MatchString(name) {
		return "", errors.New("session must be 1-64 characters of A-Z a-z 0-9 . _ -")
	}
	return filepath.Join(e.Store.StateDir("sessions"), name+".json"), nil
}

func (e *Engine) loadSession(name string) (map[string]bool, error) {
	p, err := e.sessionPath(name)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	var st struct {
		Sent []string `json:"sent"`
	}
	if json.Unmarshal(data, &st) == nil {
		for _, id := range st.Sent {
			out[id] = true
		}
	}
	return out, nil
}

func (e *Engine) saveSession(name string, sent map[string]bool) error {
	p, err := e.sessionPath(name)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(sent))
	for id := range sent {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) > 2000 {
		ids = ids[len(ids)-2000:]
	}
	data, err := json.Marshal(struct {
		Sent []string `json:"sent"`
	}{ids})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("saving session: %w", err)
	}
	return os.Rename(tmp, p)
}
