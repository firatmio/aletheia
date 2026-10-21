package mcp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firatmio/aletheia/internal/engine"
	"github.com/firatmio/aletheia/internal/store"
)

func newEngine(t *testing.T) *engine.Engine {
	t.Helper()
	root := t.TempDir()
	s, _, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return engine.New(s)
}

func exchange(t *testing.T, e *engine.Engine, lines ...string) []map[string]any {
	t.Helper()
	var out bytes.Buffer
	srv := &Server{Engine: e, Version: "test", In: strings.NewReader(strings.Join(lines, "\n") + "\n"), Out: &out}
	if err := srv.Serve(); err != nil {
		t.Fatal(err)
	}
	var res []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("server wrote invalid JSON %q: %v", l, err)
		}
		res = append(res, m)
	}
	return res
}

func toolText(t *testing.T, resp map[string]any) (string, bool) {
	t.Helper()
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result in %v", resp)
	}
	content := result["content"].([]any)
	isErr, _ := result["isError"].(bool)
	return content[0].(map[string]any)["text"].(string), isErr
}

func TestHandshakeAndTools(t *testing.T) {
	e := newEngine(t)
	resp := exchange(t, e,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","clientInfo":{"name":"claude-code"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`,
	)
	if len(resp) != 3 {
		t.Fatalf("notifications must not get replies; got %d responses", len(resp))
	}
	init := resp[0]["result"].(map[string]any)
	if init["protocolVersion"] != "2025-03-26" || init["serverInfo"].(map[string]any)["name"] != "aletheia" {
		t.Fatalf("bad initialize result: %v", init)
	}
	var names []string
	for _, tool := range resp[2]["result"].(map[string]any)["tools"].([]any) {
		names = append(names, tool.(map[string]any)["name"].(string))
	}
	if strings.Join(names, ",") != "recall,remember,get,verify" {
		t.Fatalf("tools: %v", names)
	}
}

func TestToolSchemaStaysSmall(t *testing.T) {
	// Every tool schema is sent to the model on every turn; keep the bill low.
	data, err := json.Marshal(toolDefs())
	if err != nil {
		t.Fatal(err)
	}
	if approx := len(data) / 4; approx > 600 {
		t.Fatalf("tool definitions are ~%d tokens; budget is 600", approx)
	}
}

func TestRememberRecallOverMCP(t *testing.T) {
	e := newEngine(t)
	resp := exchange(t, e,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"claude-code"}}}`,
		// lenient arguments: a single string for a list, a string for a number
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"remember","arguments":{"claim":"Go version is 1.23","anchors":"contains:go.mod::^go 1\\.23$","confidence":"0.9"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"recall","arguments":{"query":"go version","budget":"300"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"remember","arguments":{"claim":"Go version is 1.23"}}}`,
	)
	saved, isErr := toolText(t, resp[1])
	if isErr || !strings.HasPrefix(saved, "saved ") || !strings.Contains(saved, "[ok]") {
		t.Fatalf("remember: %q err=%v", saved, isErr)
	}
	id := strings.Fields(saved)[1]

	recalled, isErr := toolText(t, resp[2])
	if isErr || !strings.Contains(recalled, id+" [ok] Go version is 1.23") {
		t.Fatalf("recall: %q", recalled)
	}

	conflict, isErr := toolText(t, resp[3])
	if !isErr || !strings.Contains(conflict, "not saved") {
		t.Fatalf("duplicate should be reported as an error: %q", conflict)
	}

	g, err := e.Get(id)
	if err != nil || g.Memory.Source != "claude-code" || g.Memory.Confidence != 0.9 {
		t.Fatalf("source/confidence not recorded: %+v %v", g, err)
	}
}

func TestVerifyAndResealOverMCP(t *testing.T) {
	e := newEngine(t)
	m, _, err := e.Remember(engine.RememberInput{Claim: "Module is named x", Anchors: []string{"file:go.mod", "contains:go.mod::module x"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.Store.Root, "go.mod"), []byte("module x\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp := exchange(t, e,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"verify","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"verify","arguments":{"id":"`+m.ID+`","reseal":true}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"verify","arguments":{"reseal":true}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get","arguments":{"id":"`+m.ID+`"}}}`,
	)
	if text, _ := toolText(t, resp[0]); !strings.Contains(text, "1 stale") || !strings.Contains(text, "go.mod: changed") {
		t.Fatalf("verify all: %q", text)
	}
	if text, isErr := toolText(t, resp[1]); isErr || !strings.HasPrefix(text, "resealed") || !strings.Contains(text, "[ok]") {
		t.Fatalf("reseal: %q", text)
	}
	if _, isErr := toolText(t, resp[2]); !isErr {
		t.Fatal("reseal without id must be an error")
	}
	if text, _ := toolText(t, resp[3]); !strings.Contains(text, "anchors:") {
		t.Fatalf("get: %q", text)
	}
}

func TestProtocolErrors(t *testing.T) {
	e := newEngine(t)
	resp := exchange(t, e,
		`{not json`,
		`{"jsonrpc":"2.0","id":7,"method":"nope"}`,
		`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"explode"}}`,
		`[{"jsonrpc":"2.0","id":9,"method":"ping"}]`,
		`{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"get","arguments":{"id":"zzzzzz"}}}`,
	)
	code := func(i int) float64 { return resp[i]["error"].(map[string]any)["code"].(float64) }
	if code(0) != -32700 || resp[0]["id"] != nil {
		t.Fatalf("parse error: %v", resp[0])
	}
	if code(1) != -32601 || code(2) != -32602 || code(3) != -32600 {
		t.Fatalf("codes: %v %v %v", code(1), code(2), code(3))
	}
	if text, isErr := toolText(t, resp[4]); !isErr || !strings.Contains(text, "not found") {
		t.Fatalf("tool failures are results with isError, not protocol errors: %q", text)
	}
}
