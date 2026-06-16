# Changelog

All notable changes to hermes-remote will be documented in this file.

## [v0.1.0-a0] — 2026-06-13

### Initial Release

Remote agent for the Hermes ecosystem. Run Hermes natively on any remote machine using the main server's LLM infrastructure.

### Features

- **WebSocket Protocol** — JSON message types, 25 command types, 9 error codes, binary frame encoding for large payloads (>1MB)
- **Multi-Session Server** — WebSocket server with agent registry (JSON persistence), LLM proxy routing, per-agent session manager
- **Agent Loop** — Full Hermes agent loop (system prompt → LLM call → tool dispatch → response) using server credentials
- **Platform Interface** — Linux implementation: bash shell, xdotool (click/type/key), import/scrot (screenshot), xclip (clipboard)
- **CLI** — `--connect`, `--listen`, `--mode` (silent/interactive), `--token`, `--name` flags
- **Hermes Plugin** — `tool/plugin.py` registers 5 remote tools: `remote_agent_list`, `remote_shell`, `remote_fs_read`, `remote_fs_write`, `remote_screenshot`
- **TLS Support** — Server TLS with cert fingerprint verification
- **Cross-Compilation** — `make cross` builds for linux/amd64, linux/arm64, windows/amd64, darwin/amd64, darwin/arm64

### Fixes (post-release)

| Commit | Description |
|--------|-------------|
| `3ab97c9` | stdlib base64, process-group timeout, /health endpoint, TLS scheme detection |
| `dc02281` | append /ws path to agent connect URL if missing |

### Commits (3 total)

| # | Commit | Description |
|---|--------|-------------|
| 1 | `4c4340a` | feat: hermes-remote v0.1.0-a0 — remote agent for Hermes ecosystem |
| 2 | `3ab97c9` | fix: stdlib base64, process-group timeout, /health endpoint, TLS scheme detection |
| 3 | `dc02281` | fix: append /ws path to agent connect URL if missing |

---

## [Unreleased]

### Phase E — Production Hardening (2026-06-16)

#### Commit 1: Reconnect Hardening (`a0a20fd`)

- Replaced fixed 5s backoff in `runOutbound()` with exponential backoff + jitter
- Added `Config` fields: `MaxRetries` (0=infinite), `BackoffMin` (default 1s), `BackoffMax` (default 60s)
- Added `backoffAttempt` counter to `Agent` struct, reset on successful connection
- Added `computeBackoff()` method: `min * 2^(attempt-1)` capped at max, with jitter via `math/rand/v2`
- Wired CLI flags: `--max-retries`, `--backoff-min`, `--backoff-max`
- Zero new external deps (`math/rand/v2` is stdlib in Go 1.22)
- Build and vet pass cleanly

#### Commit 2: Windows Real Implementation (`d16dff3`)

- Replaced 6 stub functions in `platform_windows.go` with PowerShell-based real implementations
- **Screenshot**: PowerShell `System.Drawing` → PNG bytes via stdout, base64-encoded
- **ScreenInfo**: PowerShell `System.Windows.Forms.Screen::AllScreens` → parsed display list
- **Click**: PowerShell `Cursor::Position` + `user32.dll mouse_event` P/Invoke
- **TypeText**: PowerShell `SendKeys::SendWait(text)`
- **KeyPress**: PowerShell `SendKeys::SendWait({key})`
- **Hotkey**: PowerShell `SendKeys::SendWait(modifier+key)` with ctrl/alt/shift/win mapping
- **Notify**: `msg.exe` fallback → PowerShell `ToastNotificationManager` toast
- **ScreenStreamStart/Stop**: kept as stubs (deferred to Phase F)
- Added imports: `strconv`, `strings`
- Zero new external deps (PowerShell is built into Windows)
- Cross-compile: `GOOS=windows GOARCH=amd64 go build ./cmd/...` exits 0
- `go vet ./...` exits 0

#### Commit 3: macOS Real Implementation (`438ebc7`)

- **NEW** `internal/platform/platform_darwin.go` — full `darwinPlatform` with all 25 Platform interface methods using macOS-native CLI tools (zero new external deps)
- **Screenshot**: `screencapture -x -t png -` (built-in, PNG to stdout)
- **ScreenInfo**: `system_profiler SPDisplaysDataType` → parse `Resolution:` lines
- **Click/TypeText/KeyPress/Hotkey**: `osascript` AppleScript / System Events
  - `KeyPress` uses a key-code map (return/tab/space/arrows/F1–F12/home/end/pgup/pgdn)
  - `Hotkey` maps `ctrl`/`alt`/`shift`/`cmd`(win/super/meta) to System Events modifiers
- **ClipboardGet**: `pbpaste` (built-in)
- **ClipboardSet**: `pbcopy` via stdin pipe (built-in)
- **OpenURL**: `open` (built-in)
- **Notify**: `osascript display notification`
- **ProcessList**: `ps -axo pid,comm,pcpu,rss` (BSD ps; rss KB → MB)
- **ProcessKill**: `syscall.Kill(pid, signal)` (same as linux)
- **Exec**: `bash -c` with timeout via `cmd.Process.Kill()` (cross-platform safe, no `Setpgid`)
- **Filesystem (7)**: Go stdlib `os` package (cross-platform, same as linux)
- **Health**: hostname, `runtime.GOOS`/`runtime.GOARCH`, mode
- **ScreenStreamStart/Stop**: kept as stubs (deferred to Phase F)
- Added explicit `//go:build linux` build tag to `platform_linux.go` (belt-and-suspenders; the `_linux.go` filename already constrains it, but the explicit tag makes intent unambiguous and matches the windows/darwin files)
- `splitLines` duplicated in `platform_darwin.go` (no new shared file, per design)
- Zero new external deps (all macOS built-in CLI tools)
- Build verification:
  - `GOOS=darwin  GOARCH=amd64 go build ./cmd/...` exits 0
  - `GOOS=linux   GOARCH=amd64 go build ./cmd/...` exits 0 (no regression)
  - `GOOS=windows GOARCH=amd64 go build ./cmd/...` exits 0 (no regression)
  - `GOOS=darwin/linux/windows go vet ./...` exits 0

#### Commit 4: Rate Limiting (`947c306`)

- Added **per-agent token-bucket rate limiter** to the LLM proxy using stdlib only (`sync` + `time` — zero new external deps)
- **NEW** `RateLimiter` struct in `proxy.go`: per-agent `tokenBucket` map, configurable rate (tokens/sec), burst (bucket capacity), and global `maxConcurrent` (in-flight cap)
- `Allow(agentID)` — refills tokens at `rate`/sec capped at `burst`, admits if ≥1 token AND under concurrency cap; consumes a token + reserves a slot on success
- `Release()` — decrements the in-flight counter; caller must call exactly once per successful `Allow()`
- Defaults: 10 req/s, burst 20, max 5 concurrent (matching the design spec)
- **LLMProxy.Call(agentID, prompt)** — signature changed to take `agentID` for per-agent limiting; returns `("[LLM proxy: rate limited]", "rate_limited")` when denied. `SetRateLimiter()` allows overriding the default limiter
- **Server wiring**: `Server` gains a `rateLimit *RateLimiter` field; new `NewServerWithRateLimit(addr, token, registry, RateLimitConfig)` constructor applies a config to the proxy
- **CLI flags** (`cmd/server/main.go`): `--rate-limit` (float, default 10), `--rate-burst` (int, default 20), `--max-concurrent` (int, default 5). Server now uses `NewServerWithRateLimit` and logs the active limits on startup
- **NEW** `RateLimitConfig` exported struct so external packages (the server binary) can pass config to the constructor
- **NEW** `internal/server/proxy_test.go` — 6 unit tests covering: burst exhaustion, concurrency cap + Release, token refill after sleep, per-agent isolation, proxy `Call()` rate-limit marker, and `NewServerWithRateLimit` wiring. All pass.
- Build + vet + cross-compile (darwin/linux/windows × amd64/arm64) all exit 0
- `go test ./internal/server/...` passes (6/6)

#### Commit 5: Health Monitoring (`4b89328`)

- **Extended `AgentRecord`** (`internal/server/registry.go`) with: `UptimeSeconds` (computed from `ConnectedAt`), `LastError` (omitempty), `ErrorCount`, `HealthScore` (0.0–1.0 composite), `ResourceUsage *ResourceInfo` (omitempty), plus internal non-serialized `connectedAt time.Time` / `lastHeartbeat time.Time` fields (reconstructed from string fields on load via `json:"-"`-style unexported fields)
- **NEW** `ResourceInfo` struct (registry.go): `CPUPercent`, `MemoryMB`, `DiskFreeMB` — populated from agent `health_result` messages
- **Extended `protocol.HealthResult`** (`internal/protocol/messages.go`) with optional `CPUPercent`/`MemoryMB`/`DiskFreeMB` fields (omitempty) so agents can report resource usage alongside health
- **Health score formula** (documented in code comment): 0.0–1.0 composite — heartbeat recency (0.0–0.4, decays to 0 after 90s stale threshold) + error count (0.0–0.3, −0.05/error floored at 0) + uptime stability (0.0–0.3, scales over first 5 min)
- **NEW Registry methods**: `RecordError(agentID, errMsg)` (sets LastError, ++ErrorCount, recomputes score), `UpdateHealth(agentID, ResourceInfo)` (sets ResourceUsage, refreshes heartbeat, un-stales, recomputes score), `GetHealth(agentID) (AgentRecord, error)` (returns full health record with fresh uptime/score), `StartStaleDetector()` (idempotent start), `Stop()` (idempotent stop via `stopCh` + `sync.Once`)
- **Stale-detector goroutine**: scans every 30s (`staleCheckInterval`) for agents with `LastHeartbeat > 90s` old (`staleThreshold`), marks status `"stale"` and recomputes HealthScore; exits cleanly when `Registry.Stop()` closes `stopCh`. Started automatically in `NewServer`; no goroutine leak (verified: `Close()` calls `Registry.Stop()`)
- **NEW** `GET /api/agent/{id}/health` endpoint — returns the full `AgentRecord` (with `uptime_seconds`, `health_score`, `status`, `last_error`, `error_count`, `resource_usage`); 404 for unknown agent (verified: `curl` returns `agent <id> not found` with HTTP 404)
- **Enhanced `/health` endpoint** — now returns `{status, total_agents, active_agents, stale_agents, uptime_seconds}` (was just `{"status":"ok"}`)
- **Error recording wired into `handleMessages`**: incoming `TypeError` envelopes call `RecordError(agentID, env.Error.Message)`; incoming `TypeHealthResult` envelopes call `UpdateHealth` with extracted `ResourceInfo`
- Internal time fields (`connectedAt`, `lastHeartbeat`) are unexported and excluded from JSON serialization; reconstructed from the serialized string timestamps on `load()`
- Zero new external deps (stdlib `log` added for stale-detector logging)
- Build + vet pass: `go build ./cmd/...` exits 0, `go vet ./...` exits 0, `go test ./...` passes (existing 6/6 rate-limiter tests unaffected)

#### Commit 6: Token Rotation (`a66e8df`)

- **Server-side token rotation**: `Server` gains `tokenTTL time.Duration`, `tokenExpiry map[string]time.Time` (guarded by `tokenMu sync.Mutex`), `tokenStop chan struct{}`, and `tokenWG sync.WaitGroup` fields; `NewServer` initializes the map + channel
- **NEW `InitiateTokenRotation(agentID, newToken) error`** — looks up the agent's WebSocket conn under `mu.RLock`, marshals a `TokenRotateParams{NewToken, Expiry}` envelope, and sends `TypeTokenRotate` to the agent. Returns an error if the agent is not connected. Used by the proactive rotation goroutine and by the agent-initiated refresh handler
- **NEW proactive rotation goroutine** (`runTokenRotation`) — scans every 60s (`checkInterval`) for agents whose token is within 5 min of expiry (`rotationLeadTime`) and sends them a fresh token via `InitiateTokenRotation`; reschedules the next expiry relative to now. Started by `StartTokenRotation()` (no-op if `tokenTTL == 0`), called from `Start`/`StartTLS`; stopped cleanly in `Close` via `tokenStop` + `tokenWG` (no goroutine leak)
- **NEW `SetTokenTTL(ttl)`, `SetTokenExpiry(agentID, expiry)`, `ClearTokenExpiry(agentID)`** — public methods to configure and manage per-agent token expiry; expiry is set on connect and after each rotation, cleared on disconnect
- **NEW `generateToken() string`** — generates a fresh opaque token using `crypto/rand` (stdlib) → `encoding/hex` (24 bytes → 48 hex chars); falls back to a time-based token if `rand.Read` fails so rotation never blocks
- **NEW `TypeTokenRefresh = "token_refresh"`** message type (Agent → Server) — agent sends this to request a proactive refresh when its token nears expiry; server handler generates a new token, sends it via `InitiateTokenRotation`, and reschedules expiry
- **`handleMessages` switch extended**: `TypeTokenRotateResult` → parses `TokenRotateResult`, logs rotation, records `token_rotated_at` in session memory, reschedules expiry; `TypeTokenRefresh` → generates + sends new token, reschedules expiry
- **Enhanced `TokenRotateParams`** (`internal/protocol/messages.go`) with optional `Expiry time.Time` field (zero = no expiry) so the server can tell the agent when the new token expires
- **NEW `TokenRotateResult` struct** (`internal/protocol/messages.go`) with `Rotated bool` and `NewToken string` (omitempty, echo-back for confirmation) — replaces the ad-hoc `map[string]bool` the agent previously returned
- **Enhanced agent `handleTokenRotate`** (`internal/agent/agent.go`):
  - Tracks the new token in `a.cfg.Token` AND the expiry in `a.tokenExpiry` (new `Agent` field, guarded by `mu`)
  - Persists the new token to disk via `persistToken()` (writes to `a.cfg.TokenFile` with 0600 perms) so reconnects use the rotated token
  - Logs the rotation (old/new token lengths)
  - Returns a proper `TokenRotateResult{Rotated: true, NewToken: ...}` envelope (was `map[string]bool`)
- **NEW agent proactive refresh**: `handleConnection` loop gains a `refreshTicker` (60s) that checks `a.tokenExpiry`; if within 5 min of expiry it sends a `TypeTokenRefresh` envelope to the server, which responds with a new `TypeTokenRotate`
- **NEW `LoadPersistedToken(path) (string, error)`** — exported helper in the agent package; reads a persisted token from disk at startup (returns "" + nil if the file doesn't exist), used by `cmd/hermes-remote/main.go` to resume with the latest rotated token after a restart
- **CLI flag `--token-ttl duration`** (`cmd/server/main.go`, default `24h`) — server token rotation interval; `0` disables rotation. Wired via `srv.SetTokenTTL(*tokenTTL)`. Server startup log now includes `token-ttl=...`
- **CLI flag `--token-file string`** (`cmd/hermes-remote/main.go`, default `.hermes-remote-token`) — path to persist the auth token so rotated tokens survive reconnects; empty disables persistence. At startup, if no `--token` was given, the agent loads any persisted token from this file
- **`Config` gains `TokenFile string`** field; `Agent` gains `tokenExpiry time.Time` field (guarded by `mu`)
- Zero new external deps (stdlib `crypto/rand`, `encoding/hex`, `os` only). Build + vet + tests pass: `go build ./cmd/...` exits 0, `go vet ./...` exits 0, `go test ./...` passes, cross-compile (darwin/linux/windows × amd64/arm64) exits 0
- Backward compatible: existing `TypeTokenRotate` and `TokenRotateParams.NewToken` unchanged (only added optional `Expiry` field)

#### Commit 7: TLS Mutual Authentication (`201c77d`) — FINAL PHASE E COMMIT

- **TLS mutual authentication (mTLS)** — the server can now require + verify client certificates, and the agent can present a client cert when dialing a `wss://` server. Zero new external deps (`crypto/tls`, `crypto/x509` are stdlib)
- **`protocol/websocket.go` — `Dial()` enhancement**: signature gains `clientCertFile`, `clientKeyFile` params → `Dial(rawURL, certPath, clientCertFile, clientKeyFile, token)`. When both are non-empty, loads them via `tls.LoadX509KeyPair` into `TLSClientConfig.Certificates` for mTLS. Existing CA cert loading for server verification (`RootCAs`) unchanged; `InsecureSkipVerify` fallback for `wss://` without a CA cert unchanged
- **`protocol/websocket.go` — `Listen()` enhancement**: signature gains `clientCAFile` param → `Listen(addr, certFile, keyFile, clientCAFile)`. When `clientCAFile != ""`, reads the CA PEM, loads it into an `x509.CertPool`, and sets `tls.Config.ClientAuth = tls.RequireAndVerifyClientCert` + `tls.Config.ClientCAs = caCertPool`. Server rejects any client that does not present a valid cert signed by that CA
- **`protocol/server.go` — `Server` wrapper changes**: struct gains `clientCAFile string` field; `NewServer(addr, certFile, keyFile, clientCAFile)` passes it to `Listen`; `ListenAndServe()` now serves plain HTTP when no cert/key configured, TLS (with mTLS if clientCA set) otherwise
- **Removed dead `GenerateSelfSignedCert`** (`protocol/server.go`) — hardcoded PEM cert block that was never called anywhere in the codebase; also dropped the now-unused `os` import
- **`internal/server/server.go`**:
  - `Server` struct gains `certFile`, `keyFile`, `clientCAFile string` fields (stored TLS config)
  - **NEW `NewServerWithTLS(addr, token, registry, certFile, keyFile, clientCAFile)`** constructor — stores the TLS paths for `StartTLS`
  - **NEW `NewServerWithTLSRateLimit(... + RateLimitConfig)`** convenience constructor — TLS + rate limiting in one call
  - **`StartTLS(certFile, keyFile)`** now builds a `tls.Config` with `MinVersion: tls.VersionTLS13`; when `clientCAFile != ""` loads the CA pool and sets `ClientAuth = RequireAndVerifyClientCert` + `ClientCAs`. Honors the server's stored `certFile`/`keyFile` (override params may be empty). Logs `starting TLS+mTLS` vs `starting TLS` accordingly
- **`internal/agent/agent.go`**: `Config` gains `ClientCertFile`, `ClientKeyFile string` fields (client cert for mTLS on outbound `wss://`); `runOutbound()` passes them to `protocol.Dial()`
- **`cmd/server/main.go`** — NEW flags: `--cert-file` (PEM, enables TLS with `--key-file`), `--key-file` (PEM), `--client-ca` (PEM, enables mTLS, requires cert+key). When cert+key provided: uses `NewServerWithTLSRateLimit` + `StartTLS`; else `NewServerWithRateLimit` + `Start` (unchanged behavior). Startup log distinguishes TLS / TLS+mTLS / plain
- **`cmd/hermes-remote/main.go`** — NEW flags: `--client-cert` (PEM, mTLS outbound), `--client-key` (PEM, mTLS outbound), `--cert` (CA cert for server verification on outbound), `--cert-file`/`--key-file` (inbound server TLS cert/key). Wired into `agent.Config`
- **Backward compatible**: `Dial`/`Listen`/`NewServer` signature changes are internal (callers updated in the same commit). `StartTLS` still accepts `certFile, keyFile` (now optional overrides). No existing flag removed
- Build + vet + tests pass: `go build ./cmd/...` exits 0, `go vet ./...` exits 0, `go test ./...` passes (6/6 rate-limiter tests). Cross-compile clean for all 6 targets (linux/darwin/windows × amd64/arm64)

### Phase D — Kali Integration Test (2026-06-16)

| Test | Result |
|------|--------|
| Shell (`uname -a`) | ✅ PASS — Kali kernel, exit 0 |
| FS Read (`/etc/hostname`) | ✅ PASS — 5 bytes, base64 |
| FS Write | ✅ PASS — 13 bytes, disk-verified |
| Screenshot | ✅ PASS — real PNG (magic 89504e47), 233 bytes |
| Process List | ✅ PASS — 16 processes returned |
| Health | ✅ PASS — `{"status":"ok"}` |
| Registry | ✅ PASS — 1 agent `a0-kali` active |

**7/7 PASS.** Server + agent deployed to Kali Linux (100.78.148.26:2222) via SFTP. Xvfb started for display capture.

### Fixes (Phase D)

| Commit | Description |
|--------|-------------|
| `c5dde2c` | Cross-platform shell (remove Setpgid/syscall.Kill) + CODER rule #6 doc-update obligation + Windows platform stubs |
| `b25a852` | Process-list endpoint, screenshot env+PNG, --addr flag wiring |

### Bugs Found & Fixed (3)

| # | Bug | Root Cause | Fix |
|---|-----|------------|-----|
| 1 | Screenshot returned `format=error, size=0` | `import`/`scrot` didn't inherit `DISPLAY=:1` | Pass `os.Environ()` to screenshot commands |
| 2 | Process list returned `not found` | No `process-list` route in `handleAgentRoute` | Added route + `handleAgentProcessList` using `ps -eo` |
| 3 | `--addr :7705` ignored, server always on `localhost:7700` | `main.go` only read env var, no flag parsing | Added `flag.String` for `--addr`, `--token`, `--registry` with env fallback |
| 4 | Screenshot produced PostScript, not PNG | `import` defaults to PS when piping to stdout | Changed to `png:-` format specifier |

### Commits (12 total)

| # | Commit | Description |
|---|--------|-------------|
| 1 | `4c4340a` | feat: hermes-remote v0.1.0-a0 — remote agent for Hermes ecosystem |
| 2 | `3ab97c9` | fix: stdlib base64, process-group timeout, /health endpoint, TLS scheme detection |
| 3 | `dc02281` | fix: append /ws path to agent connect URL if missing |
| 4 | `c5dde2c` | fix: cross-platform shell + CODER rule #6 + Windows platform stubs |
| 5 | `b25a852` | fix: Phase D — process-list endpoint, screenshot env+PNG, --addr flag wiring |
| 6 | `a0a20fd` | feat: Phase E Commit 1 — exponential backoff + jitter reconnect hardening |
| 7 | `d16dff3` | feat: Phase E Commit 2 — Windows real PowerShell implementations (6 stubs → real) |
| 8 | `438ebc7` | feat: Phase E Commit 3 — macOS real implementation (25 functions via native CLI tools) |
| 9 | `947c306` | feat: Phase E Commit 4 — per-agent token-bucket rate limiter + CLI flags (--rate-limit/--rate-burst/--max-concurrent) |
| 10 | `4b89328` | feat: Phase E Commit 5 — health monitoring (AgentRecord extensions, stale detector, RecordError/UpdateHealth/GetHealth, /api/agent/{id}/health, enhanced /health) |
| 11 | `a66e8df` | feat: Phase E Commit 6 — token rotation (InitiateTokenRotation, proactive rotation goroutine, TokenRotateResult, agent token persistence + proactive refresh, --token-ttl, --token-file) |
| 12 | `201c77d` | feat: Phase E Commit 7 — TLS mutual authentication (mTLS): client cert loading in Dial, client CA verification in Listen, NewServerWithTLS, StartTLS mTLS, --cert-file/--key-file/--client-ca (server), --client-cert/--client-key (agent); removed dead GenerateSelfSignedCert |

### Planned
- Phase F: Final review + v1.0.0 release
