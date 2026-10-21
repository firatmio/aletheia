// Package store persists memories as Markdown files with a small,
// YAML-compatible frontmatter. It uses only the Go standard library.
package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Memory statuses stored on disk.
const (
	StatusActive     = "active"
	StatusSuperseded = "superseded"
)

var (
	idPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,31}$`)
	keyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)
)

// ValidID reports whether id is safe to use as a file name.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// Memory is one claim plus the evidence (anchors) that backs it.
type Memory struct {
	ID           string
	Claim        string
	Body         string
	Tags         []string
	Anchors      []string
	Confidence   float64
	Status       string
	Source       string
	Created      time.Time
	Updated      time.Time
	Expires      *time.Time
	Supersedes   []string
	SupersededBy string

	// extra keeps unknown frontmatter keys so that files written by newer
	// versions (or other tools) survive a read-modify-write cycle.
	extra []string
}

func quote(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimRight(buf.String(), "\n")
}

func unquote(v string) string {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, `"`) {
		var s string
		if json.Unmarshal([]byte(v), &s) == nil {
			return s
		}
	}
	if len(v) >= 2 && strings.HasPrefix(v, "'") && strings.HasSuffix(v, "'") {
		return strings.ReplaceAll(v[1:len(v)-1], "''", "'")
	}
	return v
}

func parseTime(s string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func parseInline(v string) []string {
	var out []string
	if json.Unmarshal([]byte(v), &out) == nil {
		return out
	}
	inner := strings.TrimSpace(v[1 : len(v)-1])
	if inner == "" {
		return nil
	}
	for _, part := range strings.Split(inner, ",") {
		out = append(out, unquote(part))
	}
	return out
}

func isItem(line string) bool {
	t := strings.TrimSpace(line)
	return t == "-" || strings.HasPrefix(t, "- ")
}

// Marshal renders the memory as a Markdown file.
func (m *Memory) Marshal() []byte {
	var b strings.Builder
	kv := func(k, v string) { fmt.Fprintf(&b, "%s: %s\n", k, quote(v)) }
	list := func(k string, vs []string) {
		if len(vs) == 0 {
			return
		}
		fmt.Fprintf(&b, "%s:\n", k)
		for _, v := range vs {
			fmt.Fprintf(&b, "  - %s\n", quote(v))
		}
	}
	ts := func(t time.Time) string { return t.UTC().Format(time.RFC3339) }

	b.WriteString("---\n")
	kv("id", m.ID)
	kv("claim", m.Claim)
	fmt.Fprintf(&b, "confidence: %s\n", strconv.FormatFloat(m.Confidence, 'f', -1, 64))
	status := m.Status
	if status == "" {
		status = StatusActive
	}
	kv("status", status)
	if m.Source != "" {
		kv("source", m.Source)
	}
	kv("created", ts(m.Created))
	kv("updated", ts(m.Updated))
	if m.Expires != nil {
		kv("expires", ts(*m.Expires))
	}
	list("tags", m.Tags)
	list("anchors", m.Anchors)
	list("supersedes", m.Supersedes)
	if m.SupersededBy != "" {
		kv("superseded_by", m.SupersededBy)
	}
	for _, l := range m.extra {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	b.WriteString("---\n")
	if body := strings.Trim(m.Body, "\n"); body != "" {
		b.WriteString(body)
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

// ParseMemory parses a memory file. id comes from the file name.
func ParseMemory(id string, data []byte) (*Memory, error) {
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return nil, errors.New("missing frontmatter (file must start with ---)")
	}
	rest := s[4:]
	var fm, body string
	switch {
	case strings.HasPrefix(rest, "---\n"):
		body = rest[4:]
	case rest == "---":
		// empty frontmatter, empty body
	default:
		if i := strings.Index(rest, "\n---\n"); i >= 0 {
			fm, body = rest[:i], rest[i+5:]
		} else if strings.HasSuffix(rest, "\n---") {
			fm = strings.TrimSuffix(rest, "\n---")
		} else {
			return nil, errors.New("unterminated frontmatter")
		}
	}

	m := &Memory{ID: id, Confidence: 0.8, Status: StatusActive}
	lines := strings.Split(fm, "\n")
	for i := 0; i < len(lines); {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			i++
			continue
		}
		if line[0] == ' ' || line[0] == '\t' || strings.HasPrefix(line, "- ") {
			return nil, fmt.Errorf("frontmatter line %d: unexpected %q", i+1, line)
		}
		key, val, ok := strings.Cut(line, ":")
		key = strings.TrimSpace(key)
		if !ok || !keyPattern.MatchString(key) {
			return nil, fmt.Errorf("frontmatter line %d: expected key: value", i+1)
		}
		val = strings.TrimSpace(val)
		next := i + 1
		var items []string
		switch {
		case val == "":
			for next < len(lines) && isItem(lines[next]) {
				t := strings.TrimSpace(lines[next])
				items = append(items, unquote(strings.TrimSpace(t[1:])))
				next++
			}
		case strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]"):
			items = parseInline(val)
		}

		switch key {
		case "id":
			// the file name is the source of truth
		case "claim":
			m.Claim = unquote(val)
		case "confidence":
			f, err := strconv.ParseFloat(unquote(val), 64)
			if err != nil {
				return nil, fmt.Errorf("frontmatter line %d: bad confidence", i+1)
			}
			m.Confidence = f
		case "status":
			if unquote(val) == StatusSuperseded {
				m.Status = StatusSuperseded
			}
		case "source":
			m.Source = unquote(val)
		case "created":
			m.Created = parseTime(unquote(val))
		case "updated":
			m.Updated = parseTime(unquote(val))
		case "expires":
			if t := parseTime(unquote(val)); !t.IsZero() {
				m.Expires = &t
			}
		case "superseded_by":
			m.SupersededBy = unquote(val)
		case "tags":
			m.Tags = items
		case "anchors":
			m.Anchors = items
		case "supersedes":
			m.Supersedes = items
		default:
			m.extra = append(m.extra, lines[i:next]...)
		}
		i = next
	}
	if strings.TrimSpace(m.Claim) == "" {
		return nil, errors.New("missing claim")
	}
	m.Body = strings.Trim(body, "\n")
	return m, nil
}
