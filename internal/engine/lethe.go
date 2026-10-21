package engine

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/firatmio/aletheia/internal/search"
	"github.com/firatmio/aletheia/internal/store"
)

// ParseTTL parses "30d", "2w", "12h", "90m" or any time.ParseDuration value.
// The empty string means no expiry (0).
func ParseTTL(s string) (time.Duration, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return 0, nil
	}
	unit := map[string]time.Duration{"d": 24 * time.Hour, "w": 7 * 24 * time.Hour}
	if u, ok := unit[s[len(s)-1:]]; ok && len(s) > 1 {
		n, err := strconv.ParseFloat(s[:len(s)-1], 64)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid ttl %q", s)
		}
		return time.Duration(n * float64(u)), nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("invalid ttl %q (examples: 30d, 2w, 12h)", s)
	}
	return d, nil
}

// LetheOptions controls garbage collection.
type LetheOptions struct {
	Days  int  // archive stale memories not touched for this many days (default 30)
	Apply bool // without Apply the run is a dry run
}

// LetheItem is one archive candidate.
type LetheItem struct {
	ID     string `json:"id"`
	Claim  string `json:"claim"`
	Reason string `json:"reason"`
}

// LetheReport lists what was (or would be) archived.
type LetheReport struct {
	Items    []LetheItem `json:"items"`
	Archived int         `json:"archived"`
	Applied  bool        `json:"applied"`
}

// Lethe finds superseded and long-stale memories and, with Apply, moves them
// to .aletheia/archive/. It never deletes anything.
func (e *Engine) Lethe(o LetheOptions) (*LetheReport, error) {
	if o.Days <= 0 {
		o.Days = 30
	}
	cutoff := e.now().Add(-time.Duration(o.Days) * 24 * time.Hour)
	all, _ := e.Store.List()
	rep := &LetheReport{Items: []LetheItem{}, Applied: o.Apply}
	for _, m := range all {
		var reason string
		switch {
		case m.Status == store.StatusSuperseded:
			reason = "superseded"
			if m.SupersededBy != "" {
				reason += " by " + m.SupersededBy
			}
		default:
			v := e.Judge(m)
			if v.Freshness == Stale && m.Updated.Before(cutoff) {
				reason = fmt.Sprintf("stale >%dd (%s)", o.Days, strings.Join(v.Reasons, "; "))
			}
		}
		if reason == "" {
			continue
		}
		rep.Items = append(rep.Items, LetheItem{ID: m.ID, Claim: m.Claim, Reason: reason})
		if o.Apply {
			if err := e.Store.Archive(m.ID); err != nil {
				return rep, err
			}
			rep.Archived++
		}
	}
	return rep, nil
}

// Stats summarizes the store and what recall saves compared to dumping it.
type Stats struct {
	Total        int      `json:"total"`
	Active       int      `json:"active"`
	Superseded   int      `json:"superseded"`
	Verified     int      `json:"verified"`
	Unverifiable int      `json:"unverifiable"`
	Stale        int      `json:"stale"`
	FullTokens   int      `json:"full_tokens"`  // estimated tokens to load every active memory in full
	IndexTokens  int      `json:"index_tokens"` // estimated tokens for the compact one-line index
	Problems     []string `json:"problems,omitempty"`
}

// Stats computes store statistics (token figures are estimates, ~4 chars/token).
func (e *Engine) Stats() Stats {
	all, problems := e.Store.List()
	st := Stats{Total: len(all), Problems: problems}
	for _, m := range all {
		if m.Status != store.StatusActive {
			st.Superseded++
			continue
		}
		st.Active++
		v := e.Judge(m)
		switch v.Freshness {
		case Verified:
			st.Verified++
		case Unverifiable:
			st.Unverifiable++
		default:
			st.Stale++
		}
		st.FullTokens += search.EstimateTokens(m.Claim+"\n"+m.Body) + 1
		st.IndexTokens += search.EstimateTokens(renderHit(Hit{ID: m.ID, Claim: m.Claim, Freshness: v.Freshness})) + 1
	}
	return st
}
