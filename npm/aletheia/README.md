# @firatmio/aletheia

Memory for coding agents that checks itself against your code.

Each memory is a claim plus evidence from the code: a function that must still exist, or a line that must still match. Aletheia re-checks the evidence on every recall.
- **The evidence holds:** the memory comes back as `[ok]`, with the exact lines to read.
- **The code moved unchanged:** it comes back as `[moved]`, with the new place.
- **The code changed:** the memory never reaches the agent's context.

This package installs the prebuilt `aletheia` binary for your platform: Linux, macOS or Windows, on x64 or arm64. There is no install script and no download at install time.

```bash
npm install -g @firatmio/aletheia
cd your-project
aletheia init --agents
```

As an MCP server, without a global install:

```json
{"mcpServers": {"aletheia": {"command": "npx", "args": ["-y", "@firatmio/aletheia", "serve"]}}}
```

Claude Code also has a plugin, and `go install`, Scoop and Homebrew work too. Everything is at https://aletheia.firattunaarslan.me and https://github.com/firatmio/aletheia.

Apache-2.0.
