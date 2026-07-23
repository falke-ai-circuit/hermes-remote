# PROBE — Development Guidelines

## Repository Ruleset

### Branch Strategy
- `main` — production-ready code, always builds, always passes tests
- `feature/*` — new features (e.g., `feature/vt-scan`, `feature/capabilities-toggle`)
- `fix/*` — bug fixes (e.g., `fix/terminal-scroll`)
- Branch from `main`, rebase before merge, delete after merge

### Versioning (Semantic Versioning)
- Format: `vMAJOR.MINOR.PATCH` (e.g., `v1.2.3`)
- MAJOR: breaking API or protocol changes
- MINOR: new features, new endpoints, backward-compatible
- PATCH: bug fixes, UI improvements, no new functionality
- Version bump happens in `cmd/probe-server/main.go` (`Version` const) and `cmd/probe-client/main.go`
- Tag the release: `git tag v1.2.3 && git push origin v1.2.3`

### Commit Conventions (Conventional Commits)
```
<type>(<scope>): <description>

type:    feat | fix | docs | refactor | test | chore | build | ci
scope:   server | client | web | builder | api | ui | docs
```

Examples:
- `feat(builder): add VirusTotal scan after build completion`
- `fix(web): terminal scroll auto-follow on new output`
- `feat(api): add agent capabilities toggle and redeploy endpoint`
- `docs: update README with deployment instructions`

### Before Every Commit
1. `go build ./...` — must compile clean
2. `go test ./internal/...` — all tests pass (use `-race` for CI)
3. `cd web && npm run build` — frontend builds clean
4. `git diff --cached` — review staged changes for scope creep
5. No commented-out code, no debug `fmt.Println`, no TODO without issue ref

### Before Every Push (Pre-Push Checklist)
1. All tests pass
2. Frontend built and embedded in server binary
3. CHANGELOG.md updated with version bump entry
4. If new endpoints added: update OpenAPI spec (`/openapi.json` endpoint)
5. If new config flags: update `--help` output and systemd service file
6. Delegate to architect for design docs, README updates, and repo knowledge updates
7. `git push origin main` (or feature branch)

### Architect Delegation (Before Push)
When changes affect any of these surfaces, delegate to architect before pushing:
- **README.md** — new features, deployment changes, config changes
- **DESIGN.md** — architecture changes, new subsystems, data flow changes
- **OpenAPI spec** — new or modified API endpoints
- **Repo knowledge files** — new patterns, conventions, or workflows
- **CHANGELOG.md** — version history entries

### File Organization
```
cmd/probe-server/    — server entry point, flags, wiring
cmd/probe-client/    — client/agent entry point, config embed
internal/server/     — all server-side packages (API, WS, builder, etc.)
internal/agent/      — agent-side capability handlers
internal/protocol/   — WebSocket protocol message types
internal/platform/   — platform abstraction (windows/linux/darwin)
internal/rbac/       — operators, roles, audit
internal/transfer/   — resumable file transfers
web/                 — React frontend (Vite + TypeScript)
features/            — init() package for AV evasion
embed.go             — go:embed directive for web/dist/
```

### Frontend Build
- Source: `web/src/` (TypeScript + React)
- Build: `cd web && npm run build` → outputs to `web/dist/`
- Embedded into server binary via `//go:embed all:web/dist` in `embed.go`
- **Always rebuild frontend before rebuilding server** — stale embed = stale UI

### Testing
- Unit tests: `go test ./internal/...` (with `-race` flag)
- Integration tests: `go test ./internal/server/ -run Integration`
- Frontend: `cd web && npx tsc --noEmit` (type check)
- AV test: submit to VirusTotal, expect 0/72 for clean build

### AV Evasion Recipe (Zero-Detection Build)
1. Go 1.22.5+ (use latest stable)
2. Zero build flags (NO `-trimpath`, NO `-s -w`)
3. No `.syso` PE resource file
4. `features/` package with `init()` side effects
5. `CGO_ENABLED=0` for pure Go cross-compilation
6. Expected result: 0/72 VirusTotal detections

### Cross-Compilation
```bash
# Windows amd64
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o probe-client.exe ./cmd/probe-client/

# Linux amd64 (server)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o probe-server ./cmd/probe-server/
```

### Security Rules
- Never commit credentials, tokens, or passwords
- Server credentials are bcrypt-hashed, never stored in plaintext
- Agent tokens are passed via `--token` flag or env var, never hardcoded
- `.env` files are gitignored
- API endpoints require bearer token auth (`--require-api-auth`)

### Code Style
- Go: follow `gofmt` + `go vet` clean, meaningful error wrapping with `%w`
- TypeScript: strict mode, no `any` without justification, functional components with hooks
- CSS: CSS variables in `:root`, no CSS-in-JS, class-based styling
- No magic numbers — use named constants
- Functions < 50 lines preferred, extract if longer