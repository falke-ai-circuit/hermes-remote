# ROADMAP — hermes-remote v0.1.0-a0

## Phase Overview

| Phase | Scope | Agents | Deliverable | Status |
|-------|-------|--------|-------------|--------|
| **A** | Blueprint + Roadmap | Orchestrator | BLUEPRINT.md, ROADMAP.md | ✅ Complete |
| **B** | Parallel Coding | 4 coder subagents | Protocol, Agent, Server, Plugin | ✅ Complete |
| **C** | Review + Compile + Fixes | Reviewer + Orchestrator | 8 bugs fixed, binary compiles, all 4 endpoints verified | ✅ Complete |
| **D** | Integration Test | Operative (Kali Linux) | Agent connects from Kali → server, all 7 endpoints work | ✅ Complete |
| **E** | Production Hardening | Coder + Reviewer | TLS mutual auth, token rotation, reconnect, Windows/macOS stubs | ⏳ Pending |
| **F** | Final Review + Release | Reviewer + Orchestrator | Full test suite, v1.0.0 tag, GitHub release | ⏳ Pending |

---

## Phase A — Blueprint + Roadmap ✅

- BLUEPRINT.md: architecture, protocol, components, file structure, success criteria
- ROADMAP.md: phase overview, parallel coding plan, integration test plan
- AGENTS.md, CLAUDE.md, project_knowledge.json, Makefile, .gitignore

## Phase B — Parallel Coding ✅

### B1: Protocol Layer
- `internal/protocol/` — messages.go (25 command types, 9 error codes), websocket.go (dial/listen/upgrade), binary.go (binary frame encoding), server.go (TLS server wrapper)
- Compiles independently ✅

### B2: Server Layer
- `internal/server/` — server.go (multi-session WS), registry.go (JSON persistence), proxy.go (LLM routing), session.go (per-agent Hermes session)
- Compiles, accepts connections, registers agents ✅

### B3: Agent Layer + CLI
- `internal/agent/` — agent.go (loop + dispatch)
- `internal/platform/` — platform.go (interface), platform_linux.go (bash, xdotool, import/scrot, xclip)
- `cmd/hermes-remote/main.go` — CLI flags (--connect, --listen, --mode, --token, --name)
- Compiles to binary ✅

### B4: Plugin + Tools
- `tool/plugin.py` — registers 5 remote tools: remote_agent_list, remote_shell, remote_fs_read, remote_fs_write, remote_screenshot
- `tool/plugin.yaml` — plugin manifest
- Plugin loads, tools appear in operative profile ✅

## Phase C — Review + Compile + Fixes ✅

| Check | Result |
|-------|--------|
| All packages compile | ✅ `go build ./...` exits 0 |
| go vet | ✅ All .go files pass |
| Server starts | ✅ `./server --addr :7700` binds |
| Agent connects | ✅ `./hermes-remote --connect wss://localhost:7700 --mode silent` registers |
| Plugin tools | ✅ All 5 remote_* tools registered |

### Bugs Fixed (8 across 6 files)

| # | File | Fix |
|---|------|-----|
| 1 | `internal/protocol/messages.go` | stdlib base64 instead of custom encoding |
| 2 | `internal/protocol/websocket.go` | TLS scheme detection (wss:// vs ws://) |
| 3 | `internal/agent/agent.go` | Process-group timeout for subprocess cleanup |
| 4 | `internal/server/server.go` | /health endpoint added |
| 5 | `cmd/hermes-remote/main.go` | Append /ws path to connect URL if missing |
| 6 | `tool/plugin.py` | Tool registration fixes |
| 7 | `internal/protocol/messages.go` | Message type validation |
| 8 | `internal/agent/agent.go` | Connection state machine fixes |

## Phase D — Integration Test (Kali Linux) ✅

| Test | Procedure | Pass Criteria | Result |
|------|-----------|---------------|--------|
| **D1** | Cross-compile for target arch | Binary builds clean | ✅ linux/amd64 |
| **D2** | Transfer binary to Kali | SFTP via paramiko (100.78.148.26:2222) | ✅ 8.0MB server, 8.5MB agent |
| **D3** | Start server on Kali | `./server --addr :7705` binds | ✅ health `{"status":"ok"}` |
| **D4** | Start agent in silent mode | Agent appears in registry | ✅ `a0-kali` active |
| **D5** | Shell test | `uname -a` returns Kali kernel | ✅ exit 0, 5ms |
| **D6** | FS Read test | `/etc/hostname` returns `kali` | ✅ 5 bytes, base64 |
| **D7** | FS Write test | Write + disk verify | ✅ 13 bytes, verified |
| **D8** | Screenshot test | PNG captured from Xvfb :1 | ✅ 233 bytes, magic 89504e47 |
| **D9** | Process List test | `ps -eo` returns processes | ✅ 16 processes |
| **D10** | Registry test | Agent list endpoint | ✅ 1 agent active |

**10/10 PASS.** 3 bugs found and fixed during testing (screenshot env, process-list route, --addr flag).

## Phase E — Production Hardening ⏳ (Commit 6/6 complete)

### E1: Reconnect Hardening ✅ (`a0a20fd`)
- Exponential backoff + jitter in `runOutbound()` (replaces fixed 5s)
- `Config` fields: `MaxRetries` (0=infinite), `BackoffMin` (1s), `BackoffMax` (60s)
- `computeBackoff()` method with `math/rand/v2` jitter
- CLI flags: `--max-retries`, `--backoff-min`, `--backoff-max`
- Build and vet pass cleanly

### E2: Windows Real Implementation ✅ (`d16dff3`)
- Replaced 6 stubs in `platform_windows.go` with PowerShell-based real implementations
- Screenshot (System.Drawing → PNG), ScreenInfo (AllScreens parsing), Click (Cursor + user32.dll), TypeText/KeyPress/Hotkey (SendKeys), Notify (msg.exe + ToastNotificationManager)
- ScreenStreamStart/Stop kept as stubs (deferred to Phase F)
- Added imports: `strconv`, `strings` — zero new external deps
- Cross-compile: `GOOS=windows GOARCH=amd64 go build ./cmd/...` exits 0, `go vet ./...` exits 0

### E3: macOS Real Implementation ✅ (`438ebc7`)
- **NEW** `internal/platform/platform_darwin.go` — full `darwinPlatform` implementing all 25 Platform interface methods
- macOS-native CLI tools (zero new external deps): `screencapture`, `osascript`, `pbpaste`/`pbcopy`, `open`, `ps`, `bash`
- Screenshot (`screencapture -x -t png -`), ScreenInfo (`system_profiler`), Click/TypeText/KeyPress/Hotkey (`osascript` AppleScript), Clipboard (`pbpaste`/`pbcopy`), OpenURL (`open`), Notify (`osascript display notification`), ProcessList (`ps -axo`), ProcessKill (`syscall.Kill`)
- Added explicit `//go:build linux` build tag to `platform_linux.go` (was missing; filename already constrained it, but the tag makes intent unambiguous)
- ScreenStreamStart/Stop kept as stubs (deferred to Phase F)
- Cross-compile passes for darwin/linux/windows; `go vet ./...` passes for all three GOOS

### E4: Rate Limiting ✅ (`947c306`)
- Per-agent **token-bucket** rate limiter on the LLM proxy — stdlib only (`sync` + `time`, zero new deps)
- `RateLimiter` struct: per-agent `tokenBucket` map, configurable `rate` (tokens/sec), `burst` (bucket cap), global `maxConcurrent` (in-flight cap)
- `Allow(agentID)` refills tokens at `rate`/sec (capped at `burst`), admits if ≥1 token AND under concurrency cap; `Release()` decrements in-flight counter
- Defaults: 10 req/s, burst 20, max 5 concurrent
- `LLMProxy.Call(agentID, prompt)` — signature gains `agentID`; returns `("rate_limited")` marker when denied; `SetRateLimiter()` overrides defaults
- Server: `Server.rateLimit` field + `NewServerWithRateLimit(addr, token, registry, RateLimitConfig)` constructor + exported `RateLimitConfig`
- CLI flags (`cmd/server/main.go`): `--rate-limit` (10), `--rate-burst` (20), `--max-concurrent` (5)
- 6 unit tests in `internal/server/proxy_test.go` — burst, concurrency cap + Release, token refill, per-agent isolation, proxy Call marker, constructor wiring. All pass.
- Build + vet + cross-compile (darwin/linux/windows × amd64/arm64) all exit 0

### E5: Health Monitoring ✅ (`4b89328`)
- **Extended `AgentRecord`** with `UptimeSeconds`, `LastError`, `ErrorCount`, `HealthScore` (0.0–1.0), `ResourceUsage *ResourceInfo`; internal `connectedAt`/`lastHeartbeat` `time.Time` fields excluded from JSON
- **NEW** `ResourceInfo` struct (`CPUPercent`, `MemoryMB`, `DiskFreeMB`) in registry.go; `protocol.HealthResult` extended with optional resource fields
- **Health score formula** (0.0–1.0): heartbeat recency (0.0–0.4) + error count (0.0–0.3) + uptime stability (0.0–0.3)
- **NEW Registry methods**: `RecordError`, `UpdateHealth`, `GetHealth`, `StartStaleDetector` (idempotent), `Stop` (idempotent, closes `stopCh`)
- **Stale-detector goroutine**: scans every 30s, marks agents with heartbeat > 90s as `"stale"`; started in `NewServer`, stopped in `Close` (no goroutine leak)
- **NEW** `GET /api/agent/{id}/health` — full `AgentRecord` with health fields; 404 for unknown agent
- **Enhanced `/health`** — `{status, total_agents, active_agents, stale_agents, uptime_seconds}`
- **Error recording wired** into `handleMessages` (`TypeError` → `RecordError`, `TypeHealthResult` → `UpdateHealth`)
- Zero new external deps (stdlib `log` only). Build + vet + tests pass (`go test ./...`)

### E6: Token Rotation ✅
- **Server-side token rotation**: `Server` gains `tokenTTL`, `tokenExpiry` map, `tokenStop` channel, `tokenWG`; `NewServer` initializes them
- **NEW `InitiateTokenRotation(agentID, newToken) error`** — sends `TypeTokenRotate` with `TokenRotateParams{NewToken, Expiry}` to the agent via its WebSocket conn; error if not connected
- **NEW proactive rotation goroutine** (`runTokenRotation`) — scans every 60s, rotates tokens within 5 min of expiry, reschedules next expiry. Started in `Start`/`StartTLS` (no-op if `tokenTTL == 0`), stopped in `Close` (no leak)
- **NEW `SetTokenTTL`/`SetTokenExpiry`/`ClearTokenExpiry`** + `generateToken()` (crypto/rand → hex, 48-char token)
- **NEW `TypeTokenRefresh`** message (Agent → Server) — agent requests proactive refresh; server generates + sends new token
- **`handleMessages` switch**: `TypeTokenRotateResult` → logs + records in session memory + reschedules expiry; `TypeTokenRefresh` → generates + sends new token
- **Protocol**: `TokenRotateParams` gains optional `Expiry time.Time`; NEW `TokenRotateResult{Rotated, NewToken}` struct
- **Enhanced agent `handleTokenRotate`**: updates `a.cfg.Token` + `a.tokenExpiry`, persists to `TokenFile` (0600), logs rotation, returns proper `TokenRotateResult`
- **Agent proactive refresh**: `handleConnection` gains 60s `refreshTicker`; within 5 min of expiry sends `TypeTokenRefresh`
- **NEW `LoadPersistedToken(path)`** — exported helper; agent loads persisted token at startup if no `--token` given
- **CLI flags**: `--token-ttl` (server, default 24h, 0=disabled), `--token-file` (agent, default `.hermes-remote-token`)
- Zero new deps (stdlib `crypto/rand`, `encoding/hex`, `os`). Build + vet + tests + cross-compile pass

### E7: Remaining
- TLS mutual authentication (client certs)

## Phase F — Final Review + Release ⏳

- Full integration test suite
- Security audit (no hardcoded secrets, input validation)
- Documentation completeness check
- Binary size optimization
- Git tag v1.0.0
- GitHub release with binaries for all platforms
- Plugin deployed to operative profile

---

## Timeline

| Phase | Est. Time | Status |
|-------|-----------|--------|
| A | Done | ✅ Complete |
| B | 1-2 turns (parallel) | ✅ Complete |
| C | 1 turn | ✅ Complete |
| D | 1 turn | ✅ Complete — Kali Linux (100.78.148.26) |
| E | 1-2 turns | ⏳ In Progress — Commit 5/5 complete (reconnect hardening + Windows real + macOS real + rate limiting + health monitoring) |
| F | 1 turn | ⏳ Pending |

**v0.1.0-a0 delivered: 3 commits, 2 binaries, 5 remote tools, 8 bugs fixed. Ready for Phase D integration test.**
