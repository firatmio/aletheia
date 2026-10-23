package main

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/firatmio/aletheia/internal/engine"
)

// maxTranscriptBytes skips transcripts too large to scan on every stop.
const maxTranscriptBytes = 64 << 20

// editTools are the Claude Code tools whose file_path a task changed.
var editTools = map[string]bool{"Edit": true, "Write": true, "MultiEdit": true, "NotebookEdit": true}

// captureState is what the Stop hook remembers per session between stops:
// how far it has read the transcript and the prompt being worked on.
type captureState struct {
	Offset int64  `json:"offset"`
	Prompt string `json:"prompt"`
}

type transcriptLine struct {
	Type    string `json:"type"`
	IsMeta  bool   `json:"isMeta"`
	Message struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type contentBlock struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Name  string `json:"name"`
	Input struct {
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
	} `json:"input"`
}

// captureStop runs on Claude Code's Stop hook. It reads the part of the
// transcript written since the last stop, collects the files the agent
// edited for the current prompt, and saves them as a map memory. It costs
// the agent nothing: no tool call, no tokens in its context.
func captureStop(e *engine.Engine, session, transcript string) {
	if session == "" || strings.ContainsAny(session, `/\:.`) || len(session) > 64 || transcript == "" {
		return
	}
	statePath := e.Store.StateDir("sessions", session+".capture")
	var st captureState
	if data, err := os.ReadFile(statePath); err == nil {
		_ = json.Unmarshal(data, &st)
	}
	f, err := os.Open(transcript)
	if err != nil {
		return
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || info.Size() > maxTranscriptBytes || info.Size() < st.Offset {
		return
	}
	if _, err := f.Seek(st.Offset, io.SeekStart); err != nil {
		return
	}
	var files []string
	flush := func() {
		if len(files) > 0 {
			_, _ = e.CaptureMap(st.Prompt, files)
		}
		files = nil
	}
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			break // a partial last line is read again next time
		}
		st.Offset += int64(len(line))
		var tl transcriptLine
		if json.Unmarshal(line, &tl) != nil {
			continue
		}
		switch tl.Type {
		case "user":
			// A new prompt starts a new task. One without words (only an
			// image) has nothing to name the task by, so its files are not
			// captured; they must not land on the previous prompt either.
			if p, ok := promptText(tl); ok {
				flush()
				st.Prompt = p
			}
		case "assistant":
			var blocks []contentBlock
			if json.Unmarshal(tl.Message.Content, &blocks) != nil {
				continue
			}
			for _, b := range blocks {
				if b.Type != "tool_use" || !editTools[b.Name] {
					continue
				}
				p := b.Input.FilePath
				if p == "" {
					p = b.Input.NotebookPath
				}
				if rel, ok := projectPath(e.Store.Root, p); ok {
					files = append(files, rel)
				}
			}
		}
	}
	flush()
	if data, err := json.Marshal(st); err == nil {
		if os.MkdirAll(filepath.Dir(statePath), 0o755) == nil {
			_ = os.WriteFile(statePath, data, 0o644)
		}
	}
}

// promptText returns what the person typed, and whether the message is a
// prompt from the person at all: tool results, hook output and other
// machine-written user messages are not. A prompt with only an image is one,
// with no text.
func promptText(tl transcriptLine) (string, bool) {
	if tl.IsMeta {
		return "", false
	}
	var s string
	image := false
	if json.Unmarshal(tl.Message.Content, &s) != nil {
		var blocks []contentBlock
		if json.Unmarshal(tl.Message.Content, &blocks) != nil {
			return "", false
		}
		var parts []string
		for _, b := range blocks {
			switch b.Type {
			case "tool_result":
				return "", false
			case "text":
				parts = append(parts, b.Text)
			case "image":
				image = true
			}
		}
		s = strings.Join(parts, " ")
	}
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "<") { // slash-command wrappers, system reminders
		return "", false
	}
	return s, s != "" || image
}

// projectPath turns a path from the transcript into a slash-separated path
// relative to root, refusing anything outside it.
func projectPath(root, p string) (string, bool) {
	if p == "" {
		return "", false
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}
