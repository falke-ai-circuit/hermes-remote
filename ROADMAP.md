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

## Phase E — Production Hardening ⏳ (Commit 1/4 complete)

### E1: Reconnect Hardening ✅ (`a0a20fd`)
- Exponential backoff + jitter in `runOutbound()` (replaces fixed 5s)
- `Config` fields: `MaxRetries` (0=infinite), `BackoffMin` (1s), `BackoffMax` (60s)
- `computeBackoff()` method with `math/rand/v2` jitter
- CLI flags: `--max-retries`, `--backoff-min`, `--backoff-max`
- Build and vet pass cleanly

### E2-E4: Remaining
- TLS mutual authentication (client certs)
- Token rotation (expiring tokens, refresh flow)
- Windows platform stub → real implementation
- macOS platform stub → real implementation
- Rate limiting on LLM proxy
- Agent health monitoring + alerting

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
| E | 1-2 turns | ⏳ In Progress — Commit 1/4 complete (reconnect hardening) |
| F | 1 turn | ⏳ Pending |

**v0.1.0-a0 delivered: 3 commits, 2 binaries, 5 remote tools, 8 bugs fixed. Ready for Phase D integration test.**
