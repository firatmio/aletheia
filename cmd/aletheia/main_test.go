package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/firatmio/aletheia/internal/store"
)

// TestMain keeps the tests away from the developer's own Claude Code
// settings: init reads enabledPlugins from the user's config directory.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "aletheia-claude-config")
	if err != nil {
		panic(err)
	}
	os.Setenv("CLAUDE_CONFIG_DIR", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func runCLI(t *testing.T, dir, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(append(args, "--dir", dir), strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEndToEnd(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module x\n\ngo 1.23\n")

	code, out, _ := runCLI(t, dir, "", "init", "--agents")
	if code != 0 || !strings.Contains(out, "initialized") || !strings.Contains(out, "AGENTS.md") ||
		!strings.Contains(out, "CLAUDE.md") {
		t.Fatalf("init: %d %q", code, out)
	}
	agents, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !strings.Contains(string(agents), "<!-- aletheia:begin -->") {
		t.Fatalf("AGENTS.md missing snippet: %q", agents)
	}
	claude, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if string(claude) != string(agents) {
		t.Fatalf("CLAUDE.md should get the same snippet: %q", claude)
	}
	code, out, _ = runCLI(t, dir, "", "init", "--agents")
	again, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if code != 0 || strings.Count(out, "already contains") != 2 || string(again) != string(agents) {
		t.Fatalf("init must be idempotent: %q", out)
	}

	code, out, errs := runCLI(t, dir, "", "remember", "Backend", "uses", "Go", "1.23",
		"--anchor", "file:go.mod", "--anchor", `contains:go.mod::go 1\.23`, "--tag", "backend", "--ttl", "90d")
	if code != 0 || !strings.Contains(out, "saved") || !strings.Contains(out, "[ok]") {
		t.Fatalf("remember: %d %q %q", code, out, errs)
	}
	id := strings.Fields(out)[1]

	code, out, _ = runCLI(t, dir, "", "recall", "go")
	if code != 0 || !strings.Contains(out, id+" [ok] Backend uses Go 1.23") {
		t.Fatalf("recall: %d %q", code, out)
	}

	code, _, errs = runCLI(t, dir, "", "remember", "Backend uses Go 1.23")
	if code != exitConflict || !strings.Contains(errs, "duplicate") {
		t.Fatalf("duplicate should exit %d, got %d %q", exitConflict, code, errs)
	}

	writeFile(t, dir, "go.mod", "module x\n\ngo 1.24\n")

	code, out, _ = runCLI(t, dir, "", "verify")
	if code != 1 || !strings.Contains(out, "1 stale") {
		t.Fatalf("verify should fail on stale memories: %d %q", code, out)
	}
	code, out, _ = runCLI(t, dir, "", "recall", "go")
	if code != 0 || !strings.Contains(out, "stale hidden") || strings.Contains(out, "[ok]") {
		t.Fatalf("stale memory must be hidden: %d %q", code, out)
	}
	code, out, _ = runCLI(t, dir, "", "get", id)
	if code != 0 || !strings.Contains(out, "FAIL") {
		t.Fatalf("get should show failing anchors: %d %q", code, out)
	}
	code, _, errs = runCLI(t, dir, "", "verify", id, "--reseal")
	if code != 1 || !strings.Contains(errs, "no longer holds") {
		t.Fatalf("reseal of a false claim must fail: %d %q", code, errs)
	}

	code, out, _ = runCLI(t, dir, "", "remember", "Backend uses Go 1.24",
		"--anchor", "file:go.mod", "--anchor", `contains:go.mod::go 1\.24`, "--supersedes", id)
	if code != 0 || !strings.Contains(out, "superseded: "+id) {
		t.Fatalf("supersede: %d %q", code, out)
	}
	code, out, _ = runCLI(t, dir, "", "verify")
	if code != 0 || !strings.Contains(out, "1 verified") {
		t.Fatalf("verify after supersede: %d %q", code, out)
	}

	code, out, _ = runCLI(t, dir, "", "lethe")
	if code != 0 || !strings.Contains(out, "dry run") {
		t.Fatalf("lethe: %d %q", code, out)
	}
	code, out, _ = runCLI(t, dir, "", "stats")
	if code != 0 || !strings.Contains(out, "2 total") || !strings.Contains(out, "compact index") {
		t.Fatalf("stats: %d %q", code, out)
	}
}

func TestBodyFromStdinAndJSON(t *testing.T) {
	dir := t.TempDir()
	runCLI(t, dir, "", "init")
	code, out, errs := runCLI(t, dir, "Longer notes\nover two lines\n", "remember", "Releases are cut from main", "--body", "-", "--tag", "release,ci")
	if code != 0 {
		t.Fatalf("remember: %d %q %q", code, out, errs)
	}
	id := strings.Fields(out)[1]
	_, out, _ = runCLI(t, dir, "", "get", id, "--json")
	var got struct {
		Body      string   `json:"body"`
		Tags      []string `json:"tags"`
		Freshness string   `json:"freshness"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("bad json: %v\n%s", err, out)
	}
	if got.Body != "Longer notes\nover two lines" || strings.Join(got.Tags, ",") != "release,ci" || got.Freshness != "unverifiable" {
		t.Fatalf("unexpected: %+v", got)
	}
	_, out, _ = runCLI(t, dir, "", "recall", "releases", "--json", "--full")
	var rec struct {
		Hits []struct {
			ID   string `json:"id"`
			Body string `json:"body"`
		} `json:"hits"`
	}
	if err := json.Unmarshal([]byte(out), &rec); err != nil || len(rec.Hits) != 1 || rec.Hits[0].Body == "" {
		t.Fatalf("recall --json --full: %v\n%s", err, out)
	}
}

func TestRememberSeveralFromStdin(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module x\n\ngo 1.23\n")
	runCLI(t, dir, "", "init")

	lines := `{"claim": "Go version is 1.23", "anchors": ["contains:go.mod::^go 1\\.23$"], "tags": ["toolchain"]}

{"claim": "Module path is x", "anchors": "contains:go.mod::^module x$", "ttl": "90d"}
`
	code, out, errs := runCLI(t, dir, lines, "remember", "-", "--source", "lab")
	if code != 0 || strings.Count(out, "saved ") != 2 || strings.Count(out, "[ok]") != 2 {
		t.Fatalf("remember -: %d %q %q", code, out, errs)
	}
	id := strings.Fields(out)[1]
	_, out, _ = runCLI(t, dir, "", "get", id)
	if !strings.Contains(out, "source lab") || !strings.Contains(out, "tags: toolchain") {
		t.Fatalf("source or tags not recorded: %q", out)
	}

	code, out, _ = runCLI(t, dir, `{"claim": "Go version is 1.23"}`+"\n", "remember", "-")
	if code != exitConflict || !strings.Contains(out, "#1 not saved: duplicate") {
		t.Fatalf("only conflicts should exit %d: %d %q", exitConflict, code, out)
	}

	// a regex whose \\ was collapsed to \ by the shell still parses
	code, out, _ = runCLI(t, dir, `{"claim": "Go directive names 1.23", "anchors": ["contains:go.mod::^go 1\.23$"], "force": true}`+"\n", "remember", "-")
	if code != 0 || !strings.Contains(out, "[ok] Go directive names 1.23") {
		t.Fatalf("lone backslash escape: %d %q", code, out)
	}

	// a bad line is reported by number and does not stop the others
	code, out, _ = runCLI(t, dir, "{not json\n"+`{"claim": "Releases are cut from main", "ttl": "30d"}`+"\n", "remember", "-")
	if code != exitFail || !strings.Contains(out, "#1 not saved") || !strings.Contains(out, "saved ") ||
		!strings.Contains(out, "Releases are cut from main") {
		t.Fatalf("mixed batch: %d %q", code, out)
	}

	if code, _, errs = runCLI(t, dir, "", "remember", "-"); code != exitUsage || !strings.Contains(errs, "JSON object per line") {
		t.Fatalf("empty stdin should be a usage error: %d %q", code, errs)
	}
	if code, _, errs = runCLI(t, dir, `{"claim": "Tagged elsewhere"}`, "remember", "-", "--tag", "x"); code != exitUsage ||
		!strings.Contains(errs, "--tag cannot be used with -") {
		t.Fatalf("per-memory flags with - should be a usage error: %d %q", code, errs)
	}
}

func TestUsageErrors(t *testing.T) {
	dir := t.TempDir()
	runCLI(t, dir, "", "init")
	cases := [][]string{
		{"remember"},
		{"remember", "claim here", "--bogus", "x"},
		{"recall", "--budget", "many"},
		{"verify", "--reseal"},
		{"get"},
		{"frobnicate"},
	}
	for _, args := range cases {
		if code, _, _ := runCLI(t, dir, "", args...); code != exitUsage {
			t.Errorf("%v: want exit %d, got %d", args, exitUsage, code)
		}
	}
	var out, errb bytes.Buffer
	if code := run(nil, strings.NewReader(""), &out, &errb); code != exitUsage {
		t.Errorf("no args: want exit %d, got %d", exitUsage, code)
	}
	if code := run([]string{"version"}, strings.NewReader(""), &out, &errb); code != 0 || !strings.Contains(out.String(), "aletheia") {
		t.Errorf("version: %d %q", code, out.String())
	}
}

func TestMissingStore(t *testing.T) {
	code, _, errs := runCLI(t, t.TempDir(), "", "recall", "anything")
	if code != exitFail || !strings.Contains(errs, "aletheia init") {
		t.Fatalf("want a helpful error, got %d %q", code, errs)
	}
}

func TestInitAgentsKeepsExistingClaudeMD(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "CLAUDE.md", "# Project rules\n\nUse tabs.")
	if code, out, _ := runCLI(t, dir, "", "init", "--agents"); code != 0 {
		t.Fatalf("init: %d %q", code, out)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	got := string(data)
	if !strings.HasPrefix(got, "# Project rules\n\nUse tabs.\n\n<!-- aletheia:begin -->") {
		t.Fatalf("snippet must be appended after the existing rules: %q", got)
	}
}

func TestInitClaudeHooksMergesAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, ".claude/settings.json", `{"$schema": "https://json.schemastore.org/claude-code-settings.json",
 "permissions": {"allow": ["Bash(go test:*)"]},
 "hooks": {"PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "lint-guard"}]}]}}`)

	code, out, errs := runCLI(t, dir, "", "init", "--claude-hooks")
	if code != 0 || !strings.Contains(out, "added the UserPromptSubmit and Stop hooks") {
		t.Fatalf("init --claude-hooks: %d %q %q", code, out, errs)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	var got struct {
		Schema      string `json:"$schema"`
		Permissions struct {
			Allow []string `json:"allow"`
		} `json:"permissions"`
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("settings.json is no longer valid JSON: %v\n%s", err, data)
	}
	if got.Schema == "" || len(got.Permissions.Allow) != 1 || got.Hooks["PreToolUse"][0].Hooks[0].Command != "lint-guard" {
		t.Fatalf("existing settings must be kept: %s", data)
	}
	for _, ev := range []string{"UserPromptSubmit", "Stop"} {
		if g := got.Hooks[ev]; len(g) != 1 || g[0].Hooks[0].Command != "aletheia hook" {
			t.Fatalf("%s hook missing: %s", ev, data)
		}
	}
	if s := string(data); strings.Index(s, `"$schema"`) > strings.Index(s, `"permissions"`) {
		t.Fatalf("key order must be kept: %s", s)
	}

	_, out, _ = runCLI(t, dir, "", "init", "--claude-hooks")
	again, _ := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if !strings.Contains(out, "already has the Aletheia hooks") || string(again) != string(data) {
		t.Fatalf("a second run must change nothing: %q\n%s", out, again)
	}
}

func TestInitClaudeHooksRefusesBrokenSettings(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, ".claude/settings.json", `{"hooks": `)
	if code, _, errs := runCLI(t, dir, "", "init", "--claude-hooks"); code != exitFail || !strings.Contains(errs, "settings.json") {
		t.Fatalf("broken settings must be reported, not overwritten: %d %q", code, errs)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, ".claude", "settings.json")); string(data) != `{"hooks": ` {
		t.Fatalf("broken settings were changed: %q", data)
	}
}

func TestInitPrintsNextSteps(t *testing.T) {
	dir := t.TempDir()
	_, out, _ := runCLI(t, dir, "", "init")
	if !strings.Contains(out, "next steps:") || !strings.Contains(out, "aletheia init --claude-hooks") ||
		!strings.Contains(out, "hook --dry-run") {
		t.Fatalf("init should say what is left: %q", out)
	}
	_, out, _ = runCLI(t, dir, "", "init", "--agents", "--claude-hooks")
	if strings.Contains(out, "init --claude-hooks ") || strings.Contains(out, "init --agents ") {
		t.Fatalf("steps already done should not be suggested: %q", out)
	}
}

func TestInitKnowsAboutThePluginAndSmallProjects(t *testing.T) {
	config := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", config)
	writeFile(t, config, "settings.json", `{"enabledPlugins": {"aletheia@aletheia": true}}`)
	dir := t.TempDir()
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")

	_, out, _ := runCLI(t, dir, "", "init")
	if strings.Contains(out, "init --claude-hooks") || !strings.Contains(out, "plugin is enabled and brings both hooks") {
		t.Fatalf("with the plugin enabled, init must not suggest the hooks again: %q", out)
	}
	if !strings.Contains(out, "no code map") {
		t.Fatalf("a one-file project gets no code map, and init should say so: %q", out)
	}
	_, out, _ = runCLI(t, dir, "", "init", "--claude-hooks")
	if !strings.Contains(out, "would run twice") {
		t.Fatalf("adding the hooks next to the plugin should warn: %q", out)
	}

	// A project setting that turns the plugin off wins over the user's.
	writeFile(t, dir, ".claude/settings.local.json", `{"enabledPlugins": {"aletheia@aletheia": false}}`)
	if pluginEnabled(dir) {
		t.Fatal("settings.local.json disables the plugin for this project")
	}
}

func TestDryRunSaysWhyItIsEmpty(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	runCLI(t, dir, "", "init")
	_, out, _ := runCLI(t, dir, "", "hook", "--dry-run", "how are routes structured")
	if !strings.Contains(out, "memories: none yet") || !strings.Contains(out, "code map: none for 1 source files") {
		t.Fatalf("an empty dry run should explain itself: %q", out)
	}
}

func TestEveryCommandHasHelp(t *testing.T) {
	for _, cmd := range []string{"init", "remember", "recall", "get", "verify", "lethe", "stats", "serve", "map", "hook"} {
		code, out, errs := runCLI(t, t.TempDir(), "", cmd, "--help")
		if code != 0 || !strings.Contains(out, "aletheia "+cmd) || errs != "" {
			t.Fatalf("%s --help: %d %q %q", cmd, code, out, errs)
		}
	}
	if _, out, _ := runCLI(t, t.TempDir(), "", "hook", "-h"); !strings.Contains(out, `"hook_event_name": "UserPromptSubmit"`) {
		t.Fatalf("hook help should document its stdin: %q", out)
	}
}

func TestHookDryRunChangesNothing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module x\n\ngo 1.23\n")
	runCLI(t, dir, "", "init")
	runCLI(t, dir, "", "remember", "The module targets Go 1.23", "--anchor", `contains:go.mod::^go 1\.23$`)
	before, _ := os.ReadDir(filepath.Join(dir, ".aletheia", ".state"))

	for i := 0; i < 2; i++ {
		code, out, _ := runCLI(t, dir, "", "hook", "--dry-run", "which go version does the module target")
		if code != 0 || !strings.Contains(out, "[ok] The module targets Go 1.23") || strings.Contains(out, "hookSpecificOutput") {
			t.Fatalf("dry run %d: %d %q", i, code, out)
		}
	}
	if after, _ := os.ReadDir(filepath.Join(dir, ".aletheia", ".state")); len(after) != len(before) {
		t.Fatalf("dry run must not write session state: %d → %d files", len(before), len(after))
	}
	if _, out, _ := runCLI(t, dir, "", "hook", "--dry-run", "unrelated words only"); !strings.Contains(out, "nothing would be added") {
		t.Fatalf("an empty result should say so: %q", out)
	}
}

func TestVersionNamesTheSourceBuild(t *testing.T) {
	if got := formatVersion("0.1.0", "v0.0.0-20261022130735-0d57de42de32"); got != "0.1.0 (v0.0.0-20261022130735-0d57de42de32)" {
		t.Fatalf("a build from source should say which commit: %q", got)
	}
	for _, module := range []string{"", "(devel)", "v0.1.0"} {
		if got := formatVersion("0.1.0", module); got != "0.1.0" {
			t.Fatalf("module %q: got %q", module, got)
		}
	}
}

func TestInitAgentsNotesGitIgnoredFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	writeFile(t, dir, ".gitignore", "CLAUDE.md\n")
	_, out, _ := runCLI(t, dir, "", "init", "--agents")
	if !strings.Contains(out, "git ignores CLAUDE.md here") || strings.Contains(out, "git ignores AGENTS.md") {
		t.Fatalf("init should say which instruction file git ignores: %q", out)
	}
}

func TestHookDryRunWithoutPromptShowsMemories(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module x\n\ngo 1.23\n")
	runCLI(t, dir, "", "init")
	runCLI(t, dir, "", "remember", "The module targets Go 1.23", "--anchor", `contains:go.mod::^go 1\.23$`)
	if code, out, _ := runCLI(t, dir, "", "hook", "--dry-run"); code != 0 || !strings.Contains(out, "[ok] The module targets Go 1.23") {
		t.Fatalf("dry run without a prompt: %d %q", code, out)
	}
}

func TestHookDryRunHeadingsFitThePrompt(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module x\n\ngo 1.24\n")
	writeFile(t, dir, "store.go", "package x\n\n// Store keeps memories.\ntype Store struct{}\n\nfunc (s *Store) Get() {}\n")
	runCLI(t, dir, "", "init")
	_, out, _ := runCLI(t, dir, "", "hook", "--dry-run", "--map-budget", "300")
	if !strings.Contains(out, "Code map of the project") || strings.Contains(out, "for this prompt") {
		t.Fatalf("without a prompt the map is an overview: %q", out)
	}
	_, out, _ = runCLI(t, dir, "", "hook", "--dry-run", "--map-budget", "300", "where is the store")
	if !strings.Contains(out, "Code map for this prompt") {
		t.Fatalf("with a prompt the map is fitted to it: %q", out)
	}
}

func TestHookInjectsRecallOncePerSession(t *testing.T) {
	t.Setenv("ALETHEIA_DIR", "")
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module x\n\ngo 1.23\n")
	writeFile(t, dir, "lib.go", "package x\n\nfunc Clamp(x, lo, hi int) int { return x }\n")
	runCLI(t, dir, "", "init")
	runCLI(t, dir, "", "remember", "Clamp lives in lib.go", "--anchor", "symbol:lib.go::Clamp")

	hook := func(prompt string) string {
		t.Helper()
		in, _ := json.Marshal(map[string]string{"prompt": prompt, "session_id": "s-1", "cwd": dir})
		var out, errb bytes.Buffer
		if code := run([]string{"hook"}, bytes.NewReader(in), &out, &errb); code != 0 || errb.Len() > 0 {
			t.Fatalf("hook must never fail: %d %q", code, errb.String())
		}
		return out.String()
	}
	out := hook("please make clamp faster")
	var got struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("hook output must be JSON: %v %q", err, out)
	}
	ctx := got.HookSpecificOutput.AdditionalContext
	if got.HookSpecificOutput.HookEventName != "UserPromptSubmit" ||
		!strings.Contains(ctx, "not instructions") || !strings.Contains(ctx, "[ok] Clamp lives in lib.go (lib.go:3)") {
		t.Fatalf("unexpected hook output: %+v", got)
	}
	if out := hook("clamp again"); out != "" {
		t.Fatalf("a memory is sent once per session, got %q", out)
	}
	if out := hook("nothing related"); out != "" {
		t.Fatalf("no hits means no output, got %q", out)
	}
}

func TestHookStaysQuietWithoutStoreOrInput(t *testing.T) {
	t.Setenv("ALETHEIA_DIR", "")
	for _, stdin := range []string{"", "not json", `{"prompt":"x","cwd":"` + filepath.ToSlash(t.TempDir()) + `"}`} {
		var out, errb bytes.Buffer
		if code := run([]string{"hook"}, strings.NewReader(stdin), &out, &errb); code != 0 || out.Len() > 0 || errb.Len() > 0 {
			t.Errorf("stdin %q: want silent success, got %d %q %q", stdin, code, out.String(), errb.String())
		}
	}
}

func TestHookAddsCodeMapOncePerSession(t *testing.T) {
	t.Setenv("ALETHEIA_DIR", "")
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module x\n\ngo 1.23\n")
	writeFile(t, dir, "fold.go", "package x\n\nfunc FoldConstants() {}\n")
	runCLI(t, dir, "", "init")
	hook := func(prompt string) string {
		t.Helper()
		in, _ := json.Marshal(map[string]string{"prompt": prompt, "session_id": "s-map", "cwd": dir})
		var out, errb bytes.Buffer
		run([]string{"hook", "--map-budget", "300"}, bytes.NewReader(in), &out, &errb) // the project is tiny
		return out.String()
	}
	if out := hook("fold constants in the optimizer"); !strings.Contains(out, "Code map for this prompt") ||
		!strings.Contains(out, "fold.go: FoldConstants 3") {
		t.Fatalf("first prompt should carry the code map: %q", out)
	}
	if out := hook("fold constants again"); out != "" {
		t.Fatalf("the map is sent once per session, got %q", out)
	}
	if code, out, _ := runCLI(t, dir, "", "map", "fold"); code != 0 || !strings.Contains(out, "fold.go: FoldConstants 3") {
		t.Fatalf("map command: %d %q", code, out)
	}
}

func TestHookSkipsCodeMapForTinyProjects(t *testing.T) {
	t.Setenv("ALETHEIA_DIR", "")
	dir := t.TempDir()
	writeFile(t, dir, "fold.go", "package x\n\nfunc FoldConstants() {}\n")
	runCLI(t, dir, "", "init")
	in, _ := json.Marshal(map[string]string{"prompt": "fold constants", "session_id": "s-tiny", "cwd": dir})
	var out, errb bytes.Buffer
	run([]string{"hook"}, bytes.NewReader(in), &out, &errb)
	if out.Len() != 0 {
		t.Fatalf("a one-file project needs no map: %q", out.String())
	}
}

func TestStopHookCapturesWhereTasksChangedFiles(t *testing.T) {
	t.Setenv("ALETHEIA_DIR", "")
	dir := t.TempDir()
	for _, p := range []string{"middleware", "docs"} {
		if err := os.MkdirAll(filepath.Join(dir, p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, dir, "middleware/require_header.go", "package middleware\n")
	writeFile(t, dir, "middleware/require_header_test.go", "package middleware\n")
	writeFile(t, dir, "README.md", "# x\n")
	runCLI(t, dir, "", "init")

	abs := func(p string) string { return filepath.Join(dir, filepath.FromSlash(p)) }
	lines := []any{
		map[string]any{"type": "user", "message": map[string]any{"content": "Add a RequireHeader middleware that answers 400"}},
		map[string]any{"type": "user", "isMeta": true, "message": map[string]any{"content": "hook context"}},
		map[string]any{"type": "assistant", "message": map[string]any{"content": []any{
			map[string]any{"type": "tool_use", "name": "Read", "input": map[string]any{"file_path": abs("README.md")}},
			map[string]any{"type": "tool_use", "name": "Write", "input": map[string]any{"file_path": abs("middleware/require_header.go")}},
		}}},
		map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "content": "ok"}}}},
		map[string]any{"type": "assistant", "message": map[string]any{"content": []any{
			map[string]any{"type": "tool_use", "name": "Edit", "input": map[string]any{"file_path": abs("middleware/require_header_test.go")}},
			map[string]any{"type": "tool_use", "name": "Edit", "input": map[string]any{"file_path": "README.md"}},
			map[string]any{"type": "tool_use", "name": "Edit", "input": map[string]any{"file_path": filepath.Join(filepath.Dir(dir), "outside.go")}},
			map[string]any{"type": "tool_use", "name": "Edit", "input": map[string]any{"file_path": abs("middleware/deleted.go")}},
		}}},
	}
	transcript := filepath.Join(t.TempDir(), "session.jsonl")
	appendLines := func(ls []any) {
		t.Helper()
		f, err := os.OpenFile(transcript, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		for _, l := range ls {
			data, _ := json.Marshal(l)
			f.Write(append(data, '\n'))
		}
	}
	stop := func() string {
		t.Helper()
		in, _ := json.Marshal(map[string]string{"hook_event_name": "Stop", "session_id": "s-stop", "cwd": dir, "transcript_path": transcript})
		var out, errb bytes.Buffer
		if code := run([]string{"hook"}, bytes.NewReader(in), &out, &errb); code != 0 || out.Len() > 0 || errb.Len() > 0 {
			t.Fatalf("the Stop hook must stay silent: %d %q %q", code, out.String(), errb.String())
		}
		_, recall, _ := runCLI(t, dir, "", "recall", "--limit", "20")
		return recall
	}

	appendLines(lines)
	got := stop()
	want := `[ok] Map: a task like "Add a RequireHeader middleware that answers 400" changed ` +
		`middleware/require_header.go, middleware/require_header_test.go, README.md`
	if !strings.Contains(got, want) || strings.Contains(got, "outside.go") || strings.Contains(got, "deleted.go") {
		t.Fatalf("captured map:\n%s", got)
	}
	if again := stop(); strings.Count(again, "Map:") != 1 {
		t.Fatalf("an unchanged transcript adds nothing:\n%s", again)
	}

	// The next task of the same kind replaces the captured map instead of piling up.
	writeFile(t, dir, "middleware/no_index.go", "package middleware\n")
	appendLines([]any{
		map[string]any{"type": "user", "message": map[string]any{"content": "Add a NoIndex middleware that answers with a header"}},
		map[string]any{"type": "assistant", "message": map[string]any{"content": []any{
			map[string]any{"type": "tool_use", "name": "Write", "input": map[string]any{"file_path": abs("middleware/no_index.go")}},
			map[string]any{"type": "tool_use", "name": "Edit", "input": map[string]any{"file_path": abs("README.md")}},
		}}},
	})
	got = stop()
	if strings.Count(got, "Map:") != 1 || !strings.Contains(got, "middleware/no_index.go, README.md") {
		t.Fatalf("want the newer map only:\n%s", got)
	}
}

func TestServeStartsWithoutAStore(t *testing.T) {
	// A Claude Code plugin starts the server in every project; one without
	// .aletheia/ must get a working server that says what to do.
	dir := t.TempDir()
	stdin := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"recall","arguments":{"query":"x"}}}` + "\n"
	code, out, errOut := runCLI(t, dir, stdin, "serve")
	if code != 0 || !strings.Contains(out, "aletheia init") || !strings.Contains(out, `"isError":true`) {
		t.Fatalf("serve without a store: code %d, out %q, err %q", code, out, errOut)
	}
}

func TestHookRunsOnceWhenInstalledTwice(t *testing.T) {
	// The plugin and .claude/settings.json both run `aletheia hook` for the
	// same event; the agent must get the context once and a task one map.
	t.Setenv("ALETHEIA_DIR", "")
	dir := t.TempDir()
	writeFile(t, dir, "lib.go", "package x\n\nfunc Clamp(x, lo, hi int) int { return x }\n")
	runCLI(t, dir, "", "init")
	runCLI(t, dir, "", "remember", "Clamp limits a value", "--anchor", "symbol:lib.go::Clamp")
	transcript := filepath.Join(t.TempDir(), "session.jsonl")
	writeFile(t, filepath.Dir(transcript), "session.jsonl", "")
	hook := func(event, prompt string) string {
		t.Helper()
		in, _ := json.Marshal(map[string]string{"hook_event_name": event, "prompt": prompt, "session_id": "s-twice", "cwd": dir, "transcript_path": transcript})
		var out bytes.Buffer
		run([]string{"hook"}, bytes.NewReader(in), &out, &bytes.Buffer{})
		return out.String()
	}

	// Claude Code starts both hooks at once: run them side by side.
	together := func(event, prompt string) []string {
		t.Helper()
		outs := make([]string, 4)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := range outs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				outs[i] = hook(event, prompt)
			}()
		}
		close(start)
		wg.Wait()
		return outs
	}
	sent := 0
	for _, out := range together("UserPromptSubmit", "make clamp faster") {
		if strings.Contains(out, "Clamp limits a value") {
			sent++
		}
	}
	if sent != 1 {
		t.Fatalf("simultaneous calls must add the context once, not %d times", sent)
	}

	turn, _ := json.Marshal(map[string]any{"type": "user", "message": map[string]any{"content": "make clamp faster"}})
	edit, _ := json.Marshal(map[string]any{"type": "assistant", "message": map[string]any{"content": []any{
		map[string]any{"type": "tool_use", "name": "Edit", "input": map[string]any{"file_path": filepath.Join(dir, "lib.go")}},
	}}})
	writeFile(t, filepath.Dir(transcript), "session.jsonl", string(turn)+"\n"+string(edit)+"\n")
	together("Stop", "")
	if _, out, _ := runCLI(t, dir, "", "recall", "--limit", "20"); strings.Count(out, "Map: a task like") != 1 {
		t.Fatalf("one task, one map: %q", out)
	}

	// The same prompt typed again in a later turn is a new call.
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !firstCall(st, "s-again", "UserPromptSubmit", "continue", transcript) || firstCall(st, "s-again", "UserPromptSubmit", "continue", transcript) {
		t.Fatal("the first call goes ahead, its twin does not")
	}
	writeFile(t, filepath.Dir(transcript), "session.jsonl", string(turn)+"\n"+string(edit)+"\n"+string(turn)+"\n")
	if !firstCall(st, "s-again", "UserPromptSubmit", "continue", transcript) {
		t.Fatal("a later turn has a longer transcript, so the same prompt counts again")
	}
}
