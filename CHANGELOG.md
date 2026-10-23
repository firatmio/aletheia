# Changelog

The notes for each release are in [docs/releases](docs/releases).

## 0.1.1

Packaging only; the binary behaves exactly as in 0.1.0.

- **npm:** the package declares `mcpName`, for the official MCP Registry.
- **Claude Code plugin 0.1.1:** a README, an icon and directory links, for Anthropic's plugin directory.
- **Homebrew:** the cask works on Linux.
- **Archives:** reproducible.

[Release notes](docs/releases/v0.1.1.md)

## 0.1.0 (2026-10-24)

First public release.

- **Memories with evidence:** each memory is a claim plus anchors. An anchor is one of:
  - `symbol:` a Go declaration, sealed with a fingerprint of its tokens;
  - `contains:` a line that must still match;
  - `exists:` a file that must be there;
  - `file:` a content hash.
- **Recall checks the anchors:** every recall re-checks them.
  - `[ok]` comes with the current lines.
  - `[moved]` means the code moved unchanged to another file.
  - A memory whose code changed is hidden, and `verify` says why.
- **MCP server:** `aletheia serve`, with the tools recall, remember, get and verify.
- **Claude Code:**
  - Two hooks: recall and a small code map on every prompt, and a map of the files each task changed when the task ends.
  - A plugin that installs both hooks and the MCP server.
- **Install:** npm, `go install`, Scoop, Homebrew, or the archives on the releases page.
