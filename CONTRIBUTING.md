# Contributing to Aletheia

Thanks for helping. A few things keep this project small and trustworthy.

## Ground rules

- **No new dependencies.** Aletheia uses only the Go standard library. If you think a dependency is worth it, open an issue first.
- **Anchors never execute anything.** Verification reads files inside the project root and nothing else.
- **Memory files stay human-editable.** Markdown with simple frontmatter; unknown keys must survive a read-write cycle.
- **Token cost is a feature.** Keep MCP tool descriptions short; a test fails if the tool schemas grow past ~600 tokens.

## Development

```bash
go test ./...          # all tests
go test -race ./...    # before sending a PR
go vet ./...
gofmt -l .             # must print nothing
python3 scripts/gen_assets.py   # regenerate README images after editing the script
```

## Pull requests

- One topic per PR, with tests for behaviour changes.
- Describe *why* in the PR description; the diff already shows *what*.
