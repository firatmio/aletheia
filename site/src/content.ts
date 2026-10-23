// Everything the page says lives here, so text, numbers and commands can be
// updated without touching the components. The agent endpoints (agents.ts)
// read it too.

export const site = {
  // false while the repository is private: hides the GitHub link, marks the
  // install command as not available yet and says so in /agents.md.
  released: false,
  url: "https://aletheia.firattunaarslan.me",
  repo: "https://github.com/firatmio/aletheia",
  method: "https://github.com/firatmio/aletheia/blob/main/docs/dogfood-4.2.md",
  install: "go install github.com/firatmio/aletheia/cmd/aletheia@latest",
};

export const hero = {
  eyebrow: "Memory for coding agents",
  title: ["Memory your agent", "can ", "verify."],
  lede:
    "Every memory carries evidence from your code. Aletheia re-checks it on every recall, " +
    "points your agent at the exact lines, and keeps stale facts out of its context.",
  notReleased: "The source opens soon. The command works from launch day.",
  cta: "Watch it work",
  cue: "scroll to watch an agent work with Aletheia",
  // The word behind the hero, and its dictionary entry.
  word: "ἀλήθεια",
  gloss: { translit: "alētheia", sense: "truth; literally un-forgetting, from ἀ- “not” and λήθη “forgetting”" },
};

// --- The scroll story -------------------------------------------------------
// `at` is the scroll progress (0..1 over the story) at which an item appears.
// Items with `afterRename` also wait for the rename (a click or `renameAuto`).

export type Seg = readonly [cls: string, text: string];
export type Mem = { state: "ok" | "moved" | "stale" | "map"; segs: readonly Seg[]; reason?: string };
export type Item =
  | { at: number; kind: "prompt"; segs: readonly Seg[]; afterRename?: boolean }
  | { at: number; kind: "line"; segs: readonly Seg[]; afterRename?: boolean }
  | { at: number; kind: "box"; head: string; meta?: string; mems: readonly Mem[]; afterRename?: boolean }
  | { at: number; kind: "rename" };

export const story = {
  renameAuto: 0.46,
  ghost: [0.47, 0.62] as const,
  cardAt: [0.68, 0.8] as const,
  glitch: 0.8,
  acts: [
    { id: "recall", label: "Recall in the prompt", start: 0, caption: "Every prompt starts with what still holds.",
      source: "based on lab run R5, expr-lang/expr task 4 · commands shortened" },
    { id: "move", label: "The file moves", start: 0.36, caption: "Two weeks later, a file moves.",
      source: "based on lab run R5, expr-lang/expr task 5, after the rename, as Aletheia 0.1.0-beta.4 sees it" },
    { id: "map", label: "The map writes itself", start: 0.62, caption: "Nobody wrote this down. The hook did.",
      source: "the map memory as the Stop hook writes it" },
    { id: "forget", label: "Forgetting", start: 0.8, caption: "What still holds stays. The rest is forgotten.", source: "" },
  ],
  // HUD numbers for act 1 (R5, expr task 4): with and without Aletheia.
  hud: { turns: 17, read: 21, ctrlTurns: 26, ctrlRead: 52 },
  items: [
    { at: 0, kind: "prompt", segs: [["", "Add a new builtin function "], ["ptr", "hypot(a, b)"], ["", " that returns sqrt(a*a + b*b) as a float. Type-check, document and test it like the existing math builtins."]] },
    { at: 0.05, kind: "box", head: "UserPromptSubmit · aletheia hook", meta: "3 memories · code map · 0 agent turns", mems: [
      { state: "ok", segs: [["", "New builtin: register it in the Builtins slice, validate in validation.go "], ["ptr", "builtin.go:35-1081"]] },
      { state: "ok", segs: [["", "Multi-arg builtins need an arity entry in TestBuiltin_works_with_any "], ["ptr", "builtin_test.go:263"]] },
      { state: "ok", segs: [["", "Builtins are registered in builtin/builtin.go "], ["ptr", "builtin.go:35"]] },
    ] },
    { at: 0.18, kind: "line", segs: [["dim", "● "], ["", "Read builtin/builtin.go "], ["ptr", "lines 35–50"], ["dim", " · 15 lines, not 1,081"]] },
    { at: 0.22, kind: "line", segs: [["dim", "● "], ["", "Read builtin/validation.go "], ["ptr", "lines 12–40"]] },
    { at: 0.26, kind: "line", segs: [["dim", "● "], ["", "Edit builtin/builtin.go "], ["ok", "+14"]] },
    { at: 0.29, kind: "line", segs: [["dim", "● "], ["", "Edit builtin/builtin_test.go "], ["ok", "+22"], ["dim", " · arity entry added, as the memory said"]] },
    { at: 0.32, kind: "line", segs: [["dim", "● "], ["", "Bash go test ./... "], ["ok", "ok · 14 packages"]] },

    { at: 0.41, kind: "rename" },
    { at: 0.41, kind: "prompt", afterRename: true, segs: [["", "Add a new builtin function "], ["ptr", "cbrt(x)"], ["", " that returns the cube root of a number as a float."]] },
    { at: 0.41, kind: "box", afterRename: true, head: "aletheia hook · checked against the files just now", meta: "1 moved · 1 stale hidden", mems: [
      { state: "moved", segs: [["", "New builtin: register it in the Builtins slice, validate in validation.go "], ["ptr", "registry.go:35-1081"]], reason: "the same code, now in builtin/registry.go" },
      { state: "ok", segs: [["", "Multi-arg builtins need an arity entry in TestBuiltin_works_with_any "], ["ptr", "builtin_test.go:263"]] },
      { state: "stale", segs: [["", "Builtins are registered in builtin/builtin.go"]], reason: "names builtin/builtin.go, which no longer has Builtins · hidden from your agent" },
    ] },
    { at: 0.52, kind: "line", afterRename: true, segs: [["dim", "● "], ["", "Read builtin/registry.go "], ["ptr", "lines 35–50"]] },
    { at: 0.55, kind: "line", afterRename: true, segs: [["dim", "● "], ["", "Edit builtin/registry.go "], ["ok", "+12"]] },
    { at: 0.58, kind: "line", afterRename: true, segs: [["dim", "● "], ["", "Bash go test ./... "], ["ok", "ok · 14 packages"]] },

    { at: 0.63, kind: "line", segs: [["", "⏺ Added cbrt with type checks, docs and tests."]] },
    { at: 0.66, kind: "box", head: "Stop · aletheia hook · read the session, no model call", meta: "replaced the earlier map for this kind of change", mems: [
      { state: "ok", segs: [["", "Map: a task like \"Add a new builtin function cbrt(x)…\" changed builtin/lib.go, builtin/validation.go, builtin/registry.go, builtin/builtin_test.go, docs/language-definition.md"]] },
    ] },
  ] as readonly Item[],
  renameCmd: "git mv builtin/builtin.go builtin/registry.go",
  renameChip: "Rename the file ↵",
  renameHint: "you do it, or it happens as you scroll",
  ghostLabel: "same agent · similarity memory",
  ghostNote: "illustration, not a lab run",
  ghostLines: [
    ["", "memory 0.91 Builtins are registered in builtin/builtin.go"],
    ["", "● Edit builtin/builtin.go"],
    ["bad", "✕ open builtin/builtin.go: no such file"],
    ["", "● Write builtin/builtin.go (a second Builtins slice)"],
    ["bad", "FAIL github.com/expr-lang/expr/builtin"],
    ["bad", "Builtins redeclared in this block"],
  ] as readonly Seg[],
  card: {
    file: ".aletheia/memories/<id>.md",
    lines: [
      ["dim", "---"],
      ["", "claim: \"Map: a task like …\""],
      ["", "source: \"aletheia-capture\""],
      ["", "tags: [map]"],
      ["", "anchors:"],
      ["ok", "  - exists:builtin/lib.go"],
      ["ok", "  - exists:builtin/validation.go"],
      ["ok", "  - exists:builtin/registry.go"],
      ["ok", "  - exists:builtin/builtin_test.go"],
      ["dim", "---"],
    ] as readonly Seg[],
    note: "plain Markdown · commit it, share it with your team",
  },
};

// --- Evidence board -----------------------------------------------------------

export const board = {
  eyebrow: "How it works",
  title: "Every claim is tied to the line that proves it.",
  sub: "Anchors are checked mechanically on every recall: no model calls, no embeddings, no network. When code moves to another file unchanged, its memories follow it as [moved].",
  claims: [
    {
      id: "arity",
      claim: "Multi-arg builtins need an arity entry in TestBuiltin_works_with_any",
      anchor: "symbol:builtin/builtin_test.go::TestBuiltin_works_with_any",
      file: "builtin/builtin_test.go",
      lines: [
        { n: 261, t: [["c", "// Builtins with more than one argument need an entry here."]] },
        { n: 262, t: [["k", "func"], ["", " TestBuiltin_works_with_any(t *testing.T) {"]], hit: true },
        { n: 263, t: [["", "    config := "], ["k", "map"], ["", "[string]struct{ arity "], ["k", "int"], ["", " }{"]] },
        { n: 264, t: [["", "        "], ["s", "\"clamp\""], ["", ": {3},"]] },
      ],
    },
    {
      id: "builtins",
      claim: "Builtins are registered in the Builtins slice",
      anchor: "symbol:builtin/registry.go::Builtins",
      file: "builtin/registry.go",
      editable: true,
      lines: [
        { n: 33, t: [["c", "// Builtins lists every function the language provides."]] },
        { n: 34, t: [] },
        { n: 35, t: [["k", "var"], ["", " Builtins = []*Function{"]], hit: true },
        { n: 36, t: [["", "    {"]] },
        { n: 37, t: [["", "        Name: "], ["s", "\"all\""], ["", ","]] },
      ],
    },
    {
      id: "gomod",
      claim: "go.mod targets go 1.18, so Go's min/max builtins are not available",
      anchor: "contains:go.mod::^go 1\\.18$",
      file: "go.mod",
      lines: [
        { n: 1, t: [["k", "module"], ["", " github.com/expr-lang/expr"]] },
        { n: 2, t: [] },
        { n: 3, t: [["k", "go"], ["", " 1.18"]], hit: true },
      ],
    },
  ],
  tryTitle: "Try it: break the evidence.",
  tryNote: "Runs in your browser. Nothing is sent anywhere.",
  tryLabel: "builtin/registry.go · line 35 · edit me",
  tryValue: "var Builtins = []*Function{",
  holds: "anchor holds · the memory reaches the agent",
  stale: "stale: Builtins is not declared · hidden from the agent",
};

// --- Results ------------------------------------------------------------------

export const proof = {
  eyebrow: "Results",
  title: "Measured, with the method attached.",
  figures: [
    { value: "−15%", label: "cost on gohugoio/hugo (233k lines), 14 tasks of five kinds" },
    { value: "−9%", label: "cost on expr-lang/expr, in two separate runs" },
    { value: "42/42", label: "sessions passed the hidden tests, as without Aletheia" },
  ],
  notes: [
    "R6 and R7 on Hugo, each task run with and without Aletheia by headless Claude Code (Opus 5.5): −22% on seven new template functions, −7% on a mix of new functions, helpers and behaviour changes. One run each.",
    "R5 and R6 on expr-lang/expr: the same seven tasks, −9% both times; code read before the first edit −48%.",
    "Hidden acceptance tests and the full test suite judged every session of R6 and R7.",
  ],
  caveat: "Small, tidy code saves nothing: +5% on go-chi/chi (R4).",
  methodLink: "Method and raw numbers",
  methodSoon: "Method and raw numbers are published with the source.",
};

// --- Quick start --------------------------------------------------------------

export const quickstart = {
  title: "Quick start",
  sub: "One static binary, no dependencies. Memories are plain Markdown in .aletheia/, so you can commit them and share them with your team.",
  tabs: [
    {
      id: "agent",
      label: "Let your agent do it",
      blocks: [
        { note: "Paste this into Claude Code, Cursor or any coding agent:", code: `Help me set up Aletheia in this project. The setup guide is plain text at ${site.url}/agents.md (curl -fsSL gets it exactly).`, wrap: true },
        { note: "The agent installs and initializes; the one step that changes the agent's own configuration (the hooks) it hands to you as a single command. /llms.txt indexes the guide.", code: "" },
      ],
    },
    {
      id: "claude",
      label: "Claude Code",
      blocks: [
        { note: "Install and set up the project:", code: `${site.install}\ncd your-project\naletheia init --agents --claude-hooks` },
        {
          note: "--claude-hooks merges two hooks into .claude/settings.json and keeps your other settings. Recall then happens on every prompt and costs the agent nothing. Check what an agent would get:",
          code: `aletheia hook --dry-run`,
        },
      ],
    },
    {
      id: "mcp",
      label: "Any MCP client",
      blocks: [
        {
          note: "Cursor, Cline, Codex, Claude Desktop and others: add the stdio server. It offers four tools: recall, remember, get and verify.",
          code: `{
  "mcpServers": {
    "aletheia": { "command": "aletheia", "args": ["serve"] }
  }
}`,
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
  agents: "For agents:",
  right: "Fırat Tuna Arslan",
  credit: {
    before: "Behind the title: Pierre Puvis de Chavannes, ",
    title: "The Sacred Grove, Beloved of the Arts and the Muses",
    after: " (1884–89), Art Institute of Chicago. Public domain.",
    href: "https://commons.wikimedia.org/wiki/File:Pierre_Puvis_de_Chavannes_-_The_Sacred_Grove,_Beloved_of_the_Arts_and_the_Muses_-_1922.445_-_Art_Institute_of_Chicago.jpg",
  },
};
