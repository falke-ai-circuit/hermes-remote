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

### Commits (8 total)

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

### Planned
- Phase E: Production hardening (TLS mutual auth, token rotation, Windows/macOS real implementations)
- Phase F: Final review + v1.0.0 release
