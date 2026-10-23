## What and why

<!-- The diff shows what; say why. Link the issue if there is one. -->

## Checks

- [ ] `go test -race ./...`, `go vet ./...` and `gofmt -l .` are clean
- [ ] Behaviour changes have a test
- [ ] No new dependencies (the Go standard library only)
- [ ] Anchors still never execute anything or read outside the project root
- [ ] README, `docs/` or the site updated if users see the change
