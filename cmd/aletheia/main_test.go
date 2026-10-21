package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
