# Security policy

## Supported versions

Security fixes go into the latest release. Upgrade with the same tool you installed it with: npm, `go install`, Scoop, Homebrew or the Claude Code plugin.

## Reporting a vulnerability

Please don't open a public issue. Report it privately on GitHub instead: **Security → Report a vulnerability**, or [open a draft advisory](https://github.com/firatmio/aletheia/security/advisories/new).

Include what you found, how to reproduce it, and the output of `aletheia version`. You'll get a reply within a week. The fix is released, and the advisory published, together.

## What counts

Aletheia promises a few things. Breaking any of them is a vulnerability:

- **Anchors never execute anything.** An anchor must not lead to running code.
- **Nothing is read outside the project root.** Anchor paths, symlinks included, must not escape it.
- **No network access.** The binary makes no network requests and sends no telemetry.
- **Memories are data.** A memory's text must not be able to change how Aletheia or its hooks behave.

A memory that an agent then follows like an instruction is a risk too. Report it if Aletheia's own output presents memory text as instructions.
