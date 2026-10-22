package engine

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/firatmio/aletheia/internal/anchor"
	"github.com/firatmio/aletheia/internal/search"
	"github.com/firatmio/aletheia/internal/store"
)

// CaptureSource marks memories written by CaptureMap rather than by an agent.
const CaptureSource = "aletheia-capture"

// capturePrefix starts every captured map's claim. Recall does not index it:
// "task" would otherwise match every map.
const capturePrefix = "Map: a task like "

// Limits for captured maps.
const (
	maxCapturedFiles = 8
	maxCapturedTask  = 140 // runes of the prompt kept in the claim
)

// CaptureMap records where a finished task made its changes, so the next
// task of the same kind can start at the right files. It is written from
// what the agent actually edited (no model involved, no agent turns), and
// every file gets an exists: anchor, so a rename stales the map.
//
// A new map supersedes earlier captured maps it closely resembles; it never
// replaces a memory an agent or a person wrote. It returns nil when there
// is nothing worth saving.
func (e *Engine) CaptureMap(task string, files []string) (*store.Memory, error) {
	var anchors, names []string
	seen := map[string]bool{}
	for _, f := range files {
		a, err := anchor.Make(e.Store.Root, "exists:"+f)
		if err != nil || seen[a.Path] || strings.HasPrefix(a.Path, ".aletheia/") || strings.HasPrefix(a.Path, ".claude/") {
			continue
		}
		seen[a.Path] = true
		anchors = append(anchors, a.String())
		names = append(names, a.Path)
		if len(names) == maxCapturedFiles {
			break
		}
	}
	task = shorten(strings.Join(strings.Fields(task), " "), maxCapturedTask)
	if len(names) == 0 || task == "" {
		return nil, nil
	}
	claim := fmt.Sprintf(capturePrefix+"%q changed %s", task, strings.Join(names, ", "))

	// Same kind of change = mostly the same folders and kinds of files. Such
	// earlier captured maps are replaced, so maps do not pile up per task.
	sig := changeSignature(names)
	var supersede []string
	all, _ := e.Store.List()
	for _, m := range all {
		if m.Status != store.StatusActive || m.Source != CaptureSource {
			continue
		}
		var paths []string
		for _, raw := range m.Anchors {
			if a, err := anchor.Parse(raw); err == nil && a.Kind == anchor.KindExists {
				paths = append(paths, a.Path)
			}
		}
		if search.Jaccard(sig, changeSignature(paths)) >= SimilarityThreshold {
			supersede = append(supersede, m.ID)
		}
	}
	m, _, err := e.Remember(RememberInput{
		Claim: claim, Source: CaptureSource, Tags: []string{"map"}, Anchors: anchors,
		Supersedes: supersede, Force: true,
	})
	var ce *ConflictError
	if errors.As(err, &ce) {
		return nil, nil
	}
	return m, err
}

// changeSignature describes a set of changed files by folder and kind, e.g.
// "middleware/*.go", "middleware/*_test.go", "./*.md": two tasks that add a
// middleware share it even though the file names differ.
func changeSignature(paths []string) []string {
	var out []string
	for _, p := range paths {
		dir, base := path.Split(p)
		if dir == "" {
			dir = "./"
		}
		ext := path.Ext(base)
		kind := "*" + ext
		stem := strings.TrimSuffix(base, ext)
		if strings.HasSuffix(stem, "_test") || strings.HasSuffix(stem, ".test") || strings.HasSuffix(stem, ".spec") || strings.HasPrefix(stem, "test_") {
			kind = "*_test" + ext
		}
		out = append(out, dir+kind)
	}
	return out
}

// shorten cuts s to at most n runes at a word boundary, adding "…".
func shorten(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)[:n]
	cut := string(r)
	if i := strings.LastIndexByte(cut, ' '); i > n/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,.;:") + "…"
}
