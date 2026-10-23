package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// hookCommand is what Claude Code runs for both hook events.
const hookCommand = "aletheia hook"

var hookEvents = []string{"UserPromptSubmit", "Stop"}

// installClaudeHooks adds Aletheia's UserPromptSubmit and Stop hooks to the
// project's .claude/settings.json. Other settings and hooks are kept, in
// their order, and running it again changes nothing. It is meant to be run
// by the user: agents are usually not allowed to change their own hooks.
func installClaudeHooks(root string) (string, error) {
	path := filepath.Join(root, ".claude", "settings.json")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	keys, vals, err := readObject(data)
	if err != nil {
		return "", fmt.Errorf(".claude/settings.json: %w", err)
	}
	var events []string
	groups := map[string][]json.RawMessage{}
	if raw, ok := vals["hooks"]; ok {
		ek, ev, err := readObject(raw)
		if err != nil {
			return "", fmt.Errorf(".claude/settings.json: hooks: %w", err)
		}
		events = ek
		for _, name := range ek {
			var g []json.RawMessage
			if err := json.Unmarshal(ev[name], &g); err != nil {
				return "", fmt.Errorf(".claude/settings.json: hooks.%s: expected a list", name)
			}
			groups[name] = g
		}
	}
	var added []string
	for _, name := range hookEvents {
		if hasHookCommand(groups[name]) {
			continue
		}
		if _, ok := groups[name]; !ok {
			events = append(events, name)
		}
		groups[name] = append(groups[name], json.RawMessage(`{"hooks":[{"type":"command","command":"`+hookCommand+`"}]}`))
		added = append(added, name)
	}
	if len(added) == 0 {
		return ".claude/settings.json already has the Aletheia hooks", nil
	}

	hv := map[string]json.RawMessage{}
	for _, name := range events {
		b, err := json.Marshal(groups[name])
		if err != nil {
			return "", err
		}
		hv[name] = b
	}
	if _, ok := vals["hooks"]; !ok {
		keys = append(keys, "hooks")
	}
	vals["hooks"] = writeObject(events, hv)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	out := append(writeObject(keys, vals), '\n')
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return "", err
	}
	return "added the " + strings.Join(added, " and ") + " hooks to .claude/settings.json", nil
}

func hasHookCommand(groups []json.RawMessage) bool {
	for _, g := range groups {
		var v struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		}
		if json.Unmarshal(g, &v) != nil {
			continue
		}
		for _, h := range v.Hooks {
			if c := strings.TrimSpace(h.Command); c == hookCommand || strings.HasPrefix(c, hookCommand+" ") {
				return true
			}
		}
	}
	return false
}

// readObject decodes a JSON object keeping its key order. Empty input is an
// empty object.
func readObject(data []byte) ([]string, map[string]json.RawMessage, error) {
	vals := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, vals, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, nil, errors.New("expected a JSON object")
	}
	var keys []string
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		k, _ := t.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, nil, err
		}
		if _, seen := vals[k]; !seen {
			keys = append(keys, k)
		}
		vals[k] = raw
	}
	if _, err := dec.Token(); err != nil {
		return nil, nil, err
	}
	return keys, vals, nil
}

// writeObject encodes an object in key order, indented by two spaces.
func writeObject(keys []string, vals map[string]json.RawMessage) []byte {
	var b bytes.Buffer
	b.WriteString("{")
	for i, k := range keys {
		if i > 0 {
			b.WriteString(",")
		}
		name, _ := json.Marshal(k)
		b.WriteString("\n  " + string(name) + ": ")
		var v bytes.Buffer
		if json.Indent(&v, bytes.TrimSpace(vals[k]), "  ", "  ") != nil {
			v.Reset()
			v.Write(vals[k])
		}
		b.Write(v.Bytes())
	}
	if len(keys) > 0 {
		b.WriteString("\n")
	}
	b.WriteString("}")
	return b.Bytes()
}

// pluginEnabled reports whether Aletheia's Claude Code plugin is enabled for
// the project: an "aletheia@<marketplace>" entry set to true in the
// enabledPlugins of the user's or the project's Claude Code settings. The
// first file that mentions it decides, in Claude Code's precedence: local,
// project, user. The plugin brings the hooks, so init must not suggest
// adding them again.
func pluginEnabled(root string) bool {
	config := os.Getenv("CLAUDE_CONFIG_DIR")
	if config == "" {
		if home, err := os.UserHomeDir(); err == nil {
			config = filepath.Join(home, ".claude")
		}
	}
	files := []string{
		filepath.Join(root, ".claude", "settings.local.json"),
		filepath.Join(root, ".claude", "settings.json"),
	}
	if config != "" {
		files = append(files, filepath.Join(config, "settings.json"))
	}
	for _, p := range files {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var s struct {
			EnabledPlugins map[string]any `json:"enabledPlugins"`
		}
		if json.Unmarshal(data, &s) != nil {
			continue
		}
		found, on := false, false
		for k, v := range s.EnabledPlugins {
			if strings.HasPrefix(k, "aletheia@") {
				b, _ := v.(bool)
				found, on = true, on || b
			}
		}
		if found {
			return on
		}
	}
	return false
}
