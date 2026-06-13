# AGENTS.md — hermes-remote (Agent Delegation Rules)

> This file is referenced by agent profiles when working with this repo.

## Repo Conventions

### Build
```bash
make build          # Build all binaries
make test           # Run tests (if available)
make vet            # Run go vet
make cross          # Cross-compile for all platforms
```

### Commit Style
- Prefix: `feat:`, `fix:`, `refactor:`, `docs:`, `chore:`
- Tag: `v0.1.0-a0` annotated with release notes
- Push: `git push origin main --tags`

### Forbidden
- Force-push to main without explicit approval
- Breaking proto changes without version bump
- Hardcoded secrets in source (use env vars)
- Direct edits to `go.mod`/`go.sum` (use `go get`/`go mod tidy`)

### Review Gates
1. `go build ./...` exits 0
2. `go vet ./...` passes
3. Binary runs with `--help` cleanly
4. Integration test: server starts, agent connects
