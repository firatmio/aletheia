package engine

import (
	"fmt"
	"strings"

	"github.com/firatmio/aletheia/internal/anchor"
	"github.com/firatmio/aletheia/internal/store"
)

// Label is the compact freshness marker used in all text output.
func Label(f Freshness, reasons []string) string {
	switch f {
	case Verified:
		return "ok"
	case Unverifiable:
		return "?"
	}
	if len(reasons) == 0 {
		return "stale"
	}
	return "stale: " + strings.Join(reasons, "; ")
}

func renderHit(h Hit) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s [%s] %s", h.ID, Label(h.Freshness, h.Reasons), h.Claim)
	if h.Body != "" {
		for _, l := range strings.Split(h.Body, "\n") {
			b.WriteString("\n  ")
			b.WriteString(l)
		}
	}
	return b.String()
}

// Text renders the result as compact plain text for terminals and agents.
func (r *RecallResult) Text() string {
	var parts []string
	for _, h := range r.Hits {
		parts = append(parts, h.line)
	}
	var notes []string
	if n := len(r.HiddenStale); n > 0 {
		ids := r.HiddenStale
		more := ""
		if len(ids) > 5 {
			ids, more = ids[:5], ", …"
		}
		notes = append(notes, fmt.Sprintf("+%d stale hidden: %s%s", n, strings.Join(ids, ", "), more))
	}
	if r.AlreadySent > 0 {
		notes = append(notes, fmt.Sprintf("+%d already sent", r.AlreadySent))
	}
	if r.Omitted > 0 {
		notes = append(notes, fmt.Sprintf("+%d more", r.Omitted))
	}
	if len(notes) > 0 {
		parts = append(parts, "("+strings.Join(notes, " · ")+")")
	}
	if len(parts) == 0 {
		return "no matching memories"
	}
	return strings.Join(parts, "\n")
}

// RenderSaved is the confirmation line after Remember.
func RenderSaved(m *store.Memory, v Verdict) string {
	s := fmt.Sprintf("saved %s [%s] %s", m.ID, Label(v.Freshness, v.Reasons), m.Claim)
	if len(m.Supersedes) > 0 {
		s += "\nsuperseded: " + strings.Join(m.Supersedes, ", ")
	}
	return s
}

func shortAnchor(raw string) string {
	a, err := anchor.Parse(raw)
	if err != nil || a.Kind != anchor.KindFile || len(a.Arg) != 64 {
		return raw
	}
	return "file:" + a.Path + "@" + a.Arg[:8] + "…"
}

// RenderGet renders a full memory with per-anchor status.
func RenderGet(g *GetResult) string {
	m := g.Memory
	var b strings.Builder
	fmt.Fprintf(&b, "%s [%s] %s\n", m.ID, Label(g.Verdict.Freshness, g.Verdict.Reasons), m.Claim)
	meta := []string{fmt.Sprintf("confidence %.2g", m.Confidence), "status " + m.Status}
	if !m.Updated.IsZero() {
		meta = append(meta, "updated "+m.Updated.Format("2006-01-02"))
	}
	if m.Source != "" {
		meta = append(meta, "source "+m.Source)
	}
	if m.Expires != nil {
		meta = append(meta, "expires "+m.Expires.Format("2006-01-02"))
	}
	b.WriteString(strings.Join(meta, " · "))
	if len(m.Tags) > 0 {
		b.WriteString("\ntags: " + strings.Join(m.Tags, ", "))
	}
	if m.SupersededBy != "" {
		b.WriteString("\nsuperseded by: " + m.SupersededBy)
	}
	if len(g.Anchors) > 0 {
		b.WriteString("\nanchors:")
		for _, a := range g.Anchors {
			mark := "ok"
			if !a.OK {
				mark = "FAIL " + a.Detail
			}
			fmt.Fprintf(&b, "\n  %s  %s", shortAnchor(a.Raw), mark)
		}
	}
	if m.Body != "" {
		b.WriteString("\n\n" + m.Body)
	}
	return b.String()
}

// Text renders one verification result as "id [label] claim".
func (v VerifyItem) Text() string {
	return fmt.Sprintf("%s [%s] %s", v.ID, Label(v.Freshness, v.Reasons), v.Claim)
}

// Summarize counts verification results and lists the non-verified ones.
func Summarize(items []VerifyItem) string {
	var v, u, s int
	var lines []string
	for _, it := range items {
		switch it.Freshness {
		case Verified:
			v++
		case Unverifiable:
			u++
		case Stale:
			s++
			lines = append(lines, fmt.Sprintf("%s [%s] %s", it.ID, Label(it.Freshness, it.Reasons), it.Claim))
		}
	}
	head := fmt.Sprintf("%d verified · %d unanchored · %d stale", v, u, s)
	if len(lines) == 0 {
		return head
	}
	return head + "\n" + strings.Join(lines, "\n")
}
