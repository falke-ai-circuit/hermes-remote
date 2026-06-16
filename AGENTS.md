# AGENTS.md — hermes-remote (Agent Delegation Rules)

> This file is referenced by agent profiles when working with this repo.

## Repo Conventions

### Build
```bash
make build          # Build all binaries (hermes-remote + server)
make test           # Run tests (if available)
make vet            # Run go vet
make cross          # Cross-compile for all platforms (linux/amd64, linux/arm64, windows/amd64, darwin/amd64, darwin/arm64)
```

### Commit Style
- Prefix: `feat:`, `fix:`, `refactor:`, `docs:`, `test:`, `chore:`
- Tag: annotated with release notes (`git tag -a v0.1.0-a0 -m "..."`)
- Push: `git push origin main --tags`

### Forbidden
- Force-push to main without explicit approval
- Breaking protocol changes without version bump
- Hardcoded secrets in source (use env vars)
- Direct edits to `go.mod`/`go.sum` (use `go get`/`go mod tidy`)
- New dependencies without justification in PR description

### Review Gates
1. `go build ./...` exits 0
2. `go vet ./...` passes
3. `go test ./...` passes (if tests exist)
4. Binary runs with `--help` cleanly
5. Integration test: server starts, agent connects, tool roundtrip

### R-LIVE (mandatory for server/agent changes)
- Start server binary on test port
- Start agent binary, verify connection
- Test all 5 remote tools via plugin
- Auto-re-loop on FAIL with exact failure evidence

### Creative Integration Testing
- For any deliverable talking to an external system, build a misbehaving mock and test with real I/O
- Mock server: wrong TLS cert, slow responses, connection drops
- Mock agent: invalid tokens, wrong protocol version, malformed messages

### Agent Briefs
See `.github/agents/` for per-agent task templates:
- `ANALYST.md` — Codebase analysis
- `ARCHITECT.md` — System design
- `CODER.md` — Implementation
- `REVIEWER.md` — Quality gate
- `OPERATIVE.md` — Deployment + troubleshooting
