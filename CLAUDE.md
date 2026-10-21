# CLAUDE.md

Read `PLAN.md` before doing anything: it has the project status, invariants and the ordered next steps.

## Working with Fırat

- Communicate in Turkish, friendly and concise.
- Ask before adding a dependency, changing the memory file format, or adding binary assets (GIFs, images).

## Git rules (important)

- Commits and PRs are authored as **firatmio**. Never add `Co-Authored-By: Claude …`, `Claude-Session: …` or "Generated with Claude Code" lines to commits or PR descriptions; Fırat does not want Claude attribution on this project.
- Conventional commit messages (`feat:`, `fix:`, `docs:`, `test:`, `chore:`).
- Do not push to `main` without Fırat's go-ahead once the repo exists; use branches and PRs after the initial push.

## Commands

```bash
go test ./...                 # must pass
go test -race ./...           # before committing
go vet ./...
gofmt -l .                    # must print nothing
go build -o aletheia ./cmd/aletheia
python3 scripts/gen_assets.py # after editing README images (on Windows: python scripts/gen_assets.py)
```

## Code rules

- Go standard library only. No new modules in `go.mod`.
- Anchors must never execute anything or read outside the project root.
- Keep MCP tool descriptions terse; `TestToolSchemaStaysSmall` guards the token budget.
- Every behaviour change gets a test. Prefer table-free, readable tests like the existing ones.
