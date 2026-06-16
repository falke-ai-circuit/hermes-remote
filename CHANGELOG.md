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

### Planned
- Phase D: Integration test on remote host (GWVXG74)
- Phase E: Production hardening (TLS mutual auth, token rotation, reconnect)
- Phase F: Final review + v1.0.0 release
