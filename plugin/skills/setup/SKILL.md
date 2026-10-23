---
name: setup
description: Set up Aletheia project memory in the current project. Use when the user asks to set up or initialize Aletheia, or when an Aletheia tool says no .aletheia directory was found.
---

# Set up Aletheia in this project

This plugin already provides the MCP server and the two hooks: recall on every prompt, and map capture when a task ends. What is left is the binary and the project's store.

1. Run `aletheia version`. If the command is not found, stop and give the user one of these to run, then continue once it works:
   - `go install github.com/firatmio/aletheia/cmd/aletheia@latest` (Go 1.24+)
   - `scoop bucket add firatmio https://github.com/firatmio/scoop-bucket; scoop install aletheia` (Windows)
   - `brew install --cask firatmio/tap/aletheia` (macOS, Linux)
2. In the project root, run `aletheia init --agents`. It creates `.aletheia/` and adds short usage notes to CLAUDE.md and AGENTS.md. Do not pass `--claude-hooks`: the plugin's hooks would then run twice.
3. If `.claude/settings.json` or `.claude/settings.local.json` already has `aletheia hook` entries from an earlier manual setup, tell the user and suggest removing them. Do not edit hook settings yourself.
4. Run `aletheia hook --dry-run` and show the user what the agent will receive.

Memories live in `.aletheia/memories/*.md` as plain Markdown. Suggest committing them so the team shares one memory.
