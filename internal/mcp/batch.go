package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/firatmio/aletheia/internal/engine"
)

// rememberArgs is one memory as the remember tool takes it, and as one line
// of `aletheia remember -` gives it.
type rememberArgs struct {
	Claim      string    `json:"claim"`
	Body       string    `json:"body"`
	Tags       strList   `json:"tags"`
	Anchors    strList   `json:"anchors"`
	TTL        string    `json:"ttl"`
	Supersedes strList   `json:"supersedes"`
	Force      flexBool  `json:"force"`
	Confidence flexFloat `json:"confidence"`
}

func (a rememberArgs) input(source string) (engine.RememberInput, error) {
	ttl, err := engine.ParseTTL(a.TTL)
	if err != nil {
		return engine.RememberInput{}, err
	}
	return engine.RememberInput{
		Claim: a.Claim, Body: a.Body, Source: source, Tags: a.Tags, Anchors: a.Anchors,
		Supersedes: a.Supersedes, Confidence: float64(a.Confidence), TTL: ttl, Force: bool(a.Force),
	}, nil
}

// memList is the memories argument. Models sometimes send the array as a
// JSON string, so a string holding an array is accepted too.
type memList []json.RawMessage

func (l *memList) UnmarshalJSON(b []byte) error {
	t := bytes.TrimSpace(b)
	if len(t) == 0 || string(t) == "null" {
		*l = nil
		return nil
	}
	if t[0] == '"' {
		var s string
		if err := json.Unmarshal(t, &s); err != nil {
			return err
		}
		t = []byte(s)
	}
	var out []json.RawMessage
	if err := json.Unmarshal(t, &out); err != nil {
		return errors.New("memories: expected an array of objects")
	}
	*l = out
	return nil
}

// looseEscapes doubles backslashes that do not start a JSON escape, so a
// regex written as "^go 1\.23$" still parses: shells and tool layers often
// collapse \\ to \ before a line reaches us. Valid JSON never needs this.
func looseEscapes(b []byte) []byte {
	out := make([]byte, 0, len(b)+8)
	for i := 0; i < len(b); i++ {
		out = append(out, b[i])
		if b[i] != '\\' {
			continue
		}
		if i+1 < len(b) && strings.IndexByte(`"\/bfnrtu`, b[i+1]) >= 0 {
			i++
			out = append(out, b[i])
		} else {
			out = append(out, '\\')
		}
	}
	return out
}

// BatchResult is the outcome of RememberBatch.
type BatchResult struct {
	Text      string // one result per memory, in order
	Saved     int
	Conflicts int // not saved because a duplicate or similar memory exists
	Failed    int // not saved for any other reason
}

// RememberBatch saves each memory (a JSON object with the remember tool's
// arguments) on its own and reports one result per memory, so one bad
// memory does not cost the others. force applies to all of them.
func RememberBatch(e *engine.Engine, raws []json.RawMessage, source string, force bool) BatchResult {
	var r BatchResult
	lines := make([]string, 0, len(raws))
	for i, raw := range raws {
		err := func() error {
			var a rememberArgs
			if err := json.Unmarshal(raw, &a); err != nil {
				if json.Unmarshal(looseEscapes(raw), &a) != nil {
					return err
				}
			}
			in, err := a.input(source)
			if err != nil {
				return err
			}
			in.Force = in.Force || force
			m, v, err := e.Remember(in)
			if err != nil {
				return err
			}
			lines = append(lines, engine.RenderSaved(m, v))
			return nil
		}()
		var ce *engine.ConflictError
		switch {
		case err == nil:
			r.Saved++
			continue
		case errors.As(err, &ce):
			r.Conflicts++
		default:
			r.Failed++
		}
		lines = append(lines, fmt.Sprintf("#%d not saved: %v", i+1, err))
	}
	r.Text = strings.Join(lines, "\n")
	return r
}
