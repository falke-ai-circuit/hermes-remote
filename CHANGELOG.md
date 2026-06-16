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

### Commits (5 total)

| # | Commit | Description |
|---|--------|-------------|
| 1 | `4c4340a` | feat: hermes-remote v0.1.0-a0 — remote agent for Hermes ecosystem |
| 2 | `3ab97c9` | fix: stdlib base64, process-group timeout, /health endpoint, TLS scheme detection |
| 3 | `dc02281` | fix: append /ws path to agent connect URL if missing |
| 4 | `c5dde2c` | fix: cross-platform shell + CODER rule #6 + Windows platform stubs |
| 5 | `b25a852` | fix: Phase D — process-list endpoint, screenshot env+PNG, --addr flag wiring |

### Planned
- Phase E: Production hardening (TLS mutual auth, token rotation, reconnect)
- Phase F: Final review + v1.0.0 release
