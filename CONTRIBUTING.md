# Contributing to PROBE

## Commit Conventions

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat:     New feature
fix:      Bug fix
refactor: Code change that neither fixes a bug nor adds a feature
docs:     Documentation only
test:     Adding or updating tests
chore:    Build process, tooling, CI
```

## Pull Request Process

1. Create a feature branch from `main`
2. Make changes with conventional commit messages
3. Ensure `go build ./...` and `go vet ./...` pass
4. Ensure `go test ./...` passes (if tests exist)
5. Open PR against `main`
6. PR must pass CI (build.yml) before merge

## Review Requirements

- No force-push to `main` without explicit approval
- No breaking protocol changes without version bump
- No hardcoded secrets (use environment variables)
- Direct edits to `go.mod`/`go.sum` only via `go get`/`go mod tidy`
- New dependencies require justification in PR description

## Development Setup

```bash
# Clone
git clone https://github.com/falke-ai-circuit/probe.git
cd PROBE

# Build
make build

# Test
make test

# Cross-compile
make cross
```

## Project Structure

```
probe/
├── cmd/
│   ├── probe-client/    # Agent binary
│   └── server/           # Server binary
├── internal/
│   ├── agent/            # Agent loop, connection management
│   ├── platform/         # Platform interface (Linux implementation)
│   ├── protocol/         # WebSocket protocol, messages, TLS
│   └── server/           # Multi-session server, registry, proxy
├── tool/
│   ├── plugin.py         # Hermes plugin (5 remote tools)
│   └── plugin.yaml       # Plugin manifest
├── AGENTS.md             # Agent delegation rules
├── CLAUDE.md             # Project overview + build/run instructions
├── project_knowledge.json # Hot cache + architecture map + gotchas
├── BLUEPRINT.md          # Operational blueprint
├── ROADMAP.md            # Phase overview + timeline
├── CHANGELOG.md          # Release history
├── README.md             # Project overview
├── Makefile              # Build, test, cross-compile
└── .github/workflows/    # CI
```
