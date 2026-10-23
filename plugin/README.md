# Aletheia for Claude Code

Aletheia is project memory that checks itself against your code. Each memory is a claim plus anchors: a function that must still exist, a line that must still match, a file that must be there. On every prompt the plugin recalls the memories that fit and re-checks their anchors. A memory whose evidence holds comes back as `[ok]` with the exact lines to read, one whose code moved unchanged comes back as `[moved]` with the new place, and one whose code changed never reaches Claude's context.

## Requirements

The plugin runs the `aletheia` binary from your PATH; it does not download anything. Install it once, any of these ways:

- `npm install -g @firatmio/aletheia`
- `go install github.com/firatmio/aletheia/cmd/aletheia@latest`
- `scoop bucket add firatmio https://github.com/firatmio/scoop-bucket` then `scoop install aletheia`
- `brew install --cask firatmio/tap/aletheia`

Then run `/aletheia:setup` in a project. It runs `aletheia init --agents`, which creates `.aletheia/` and adds short usage notes to CLAUDE.md and AGENTS.md.

## What the plugin runs

- **UserPromptSubmit hook:** `aletheia hook`. Reads the prompt, the project's `.aletheia/` memories and the project's source files, and adds the verified memories and a small code map to Claude's context.
- **Stop hook:** `aletheia hook`. Reads the session transcript file that Claude Code names, and records which project files the task edited as a Markdown memory in `.aletheia/`. That memory is labelled with the task's prompt, shortened to 140 characters. It stays in the project, on your machine.
- **MCP server:** `aletheia serve` over stdio, with the tools `recall`, `remember`, `get` and `verify`.
- **Skill:** `/aletheia:setup`, the setup steps above.

Everything stays on your machine. Aletheia makes no network requests and sends no telemetry. It reads and writes only inside the project: anchors never execute anything and never read outside the project root. The one program it starts is `git check-ignore`, during `init`, to see whether CLAUDE.md is ignored.

If you also added the hooks by hand (`aletheia init --claude-hooks`), remove them from `.claude/settings.json`; with the plugin they would otherwise be listed twice. Aletheia runs a hook once per event either way.

Source, method and measurements: https://github.com/firatmio/aletheia · Website: https://aletheia.firattunaarslan.me · License: Apache-2.0
