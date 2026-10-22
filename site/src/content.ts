// Everything the page says lives here, so text, numbers and commands can be
// updated without touching the components.

export const site = {
  // false while the repository is private: hides the GitHub link and marks
  // the install command as not available yet.
  released: false,
  repo: "https://github.com/firatmio/aletheia",
  method: "https://github.com/firatmio/aletheia/blob/main/docs/dogfood-4.2.md",
  install: "go install github.com/firatmio/aletheia/cmd/aletheia@latest",
};

export const hero = {
  eyebrow: "Memory for coding agents",
  title: ["Memory your agent can ", "verify."],
  lede:
    "Every memory carries evidence from your code. Aletheia re-checks it on every recall, " +
    "points your agent at the exact lines, and keeps stale facts out of its context.",
  notReleased: "The source opens soon. The command works from launch day.",
};

// The hero demo: the same recall before and after a file is renamed.
export const demo = {
  query: 'aletheia recall "add a builtin"',
  event: "git mv builtin/builtin.go builtin/registry.go",
  before: {
    label: "Before the rename",
    file: "builtin/builtin.go",
    memories: [
      { state: "ok", claim: "Builtins are registered in the Builtins slice", ptr: "builtin.go:35-1081" },
      { state: "ok", claim: "Multi-arg builtins need an arity entry in TestBuiltin_works_with_any", ptr: "builtin_test.go:263" },
    ],
    anchor: "symbol:builtin/builtin.go::Builtins",
  },
  after: {
    label: "After the rename",
    file: "builtin/registry.go",
    memories: [
      { state: "ok", claim: "New builtin: register it in the Builtins slice, validate in validation.go", ptr: "registry.go:35-1081" },
      { state: "ok", claim: "Multi-arg builtins need an arity entry in TestBuiltin_works_with_any", ptr: "builtin_test.go:263" },
      { state: "stale", claim: "Builtins are registered in the Builtins slice", reason: "builtin/builtin.go: missing · hidden from your agent" },
    ],
    anchor: "symbol:builtin/registry.go::Builtins",
  },
  code: [
    { n: 33, t: [["c", "// Builtins lists every function the language provides."]] },
    { n: 34, t: [] },
    { n: 35, hit: true, t: [["k", "var"], ["", " Builtins = []*Function{"]] },
    { n: 36, hit: true, t: [["", "    {"]] },
    { n: 37, hit: true, t: [["", "        Name: "], ["s", '"all"'], ["", ","]] },
    { n: 38, hit: true, t: [["", "        Predicate: "], ["k", "true"], ["", ","]] },
    { n: 39, hit: true, t: [["", "    },"]] },
  ],
} as const;

export const proof = {
  figures: [
    { value: "−12%", label: "cost per task sequence on expr-lang/expr" },
    { value: "−60%", label: "code read before the first edit, first task" },
    { value: "12/12", label: "tasks passed, same as without Aletheia" },
  ],
  note:
    "Headless Claude Code (Opus 5.5), the same tasks with and without Aletheia, judged by the same hidden tests. " +
    "Small, tidy packages see no saving: +5% on go-chi/chi.",
  methodLink: "Read the method",
  methodSoon: "Method and raw numbers are published with the source.",
};

export const problem = {
  title: "A confident wrong memory is worse than none",
  text:
    "Memory tools rank by similarity, and an outdated memory looks just as relevant as the fact that replaced it. " +
    "The agent then builds on something that stopped being true weeks ago.",
  naive: {
    label: "Similarity memory",
    rows: [
      { score: "0.91", claim: "Builtins are registered in builtin/builtin.go" },
      { score: "0.89", claim: "Builtins are registered in builtin/registry.go" },
    ],
    verdict: "Both look true. One is not.",
  },
  verified: {
    label: "Aletheia",
    rows: [{ state: "ok", claim: "Builtins are registered in builtin/registry.go", ptr: "registry.go:35" }],
    footer: "1 stale memory hidden: builtin/builtin.go is missing",
    verdict: "The agent only sees what still holds.",
  },
};

export const how = {
  title: "How it works",
  steps: [
    {
      title: "Memories carry evidence",
      text: "Each memory is a claim plus anchors: a function that must still exist, a line that must still match, a file that must still be there.",
    },
    {
      title: "Evidence is checked on every recall",
      text: "The checks are mechanical: no model calls, no embeddings, no network. A memory whose evidence broke is hidden, and a one-line note says so.",
    },
    {
      title: "Recall points at lines",
      text: "A pointer such as builtin_test.go:263 is computed from the files right now, so the agent opens the right lines instead of searching.",
    },
    {
      title: "Maps write themselves",
      text: "When the agent stops, Aletheia records which files the task changed. The next task of the same kind starts there, at no cost in turns.",
    },
  ],
  anchorsTitle: "Anchors",
  anchors: [
    { kind: "symbol", holds: "the file still declares that function, method, type, var or const", example: "symbol:store.go::Store.Get" },
    { kind: "contains", holds: "a line in the file still matches the regular expression", example: "contains:go.mod::^go 1\\.23$" },
    { kind: "exists", holds: "the path is still there", example: "exists:internal/auth" },
    { kind: "file", holds: "the file is byte-for-byte unchanged (sealed hash)", example: "file:go.mod" },
  ],
};

export const quickstart = {
  title: "Quick start",
  sub: "One static binary, no dependencies. Memories are plain Markdown in .aletheia/, so you can commit them and share them with your team.",
  tabs: [
    {
      id: "claude",
      label: "Claude Code",
      blocks: [
        {
          note: "Install and set up the project:",
          code: "go install github.com/firatmio/aletheia/cmd/aletheia@latest\ncd your-project\naletheia init --agents",
        },
        {
          note: "Add the two hooks to .claude/settings.json. Recall then happens on every prompt and costs the agent nothing:",
          code: `{
  "hooks": {
    "UserPromptSubmit": [{ "hooks": [{ "type": "command", "command": "aletheia hook" }] }],
    "Stop": [{ "hooks": [{ "type": "command", "command": "aletheia hook" }] }]
  }
}`,
        },
      ],
    },
    {
      id: "mcp",
      label: "Any MCP client",
      blocks: [
        {
          note: "Cursor, Cline, Codex, Claude Desktop and others: add the stdio server.",
          code: `{
  "mcpServers": {
    "aletheia": { "command": "aletheia", "args": ["serve"] }
  }
}`,
        },
        {
          note: "It offers four tools with small schemas: recall, remember, get and verify.",
          code: "",
        },
      ],
    },
    {
      id: "cli",
      label: "CLI",
      blocks: [
        {
          note: "Any agent that can run a shell command can use it:",
          code: `aletheia remember "Builtins are registered in the Builtins slice" \\
  --anchor symbol:builtin/registry.go::Builtins
aletheia recall "add a builtin"
aletheia verify            # exits 1 if anything is stale`,
        },
      ],
    },
  ],
};

export const footer = {
  left: "Apache-2.0 · written in Go, zero dependencies",
  right: "Fırat Tuna Arslan",
};
