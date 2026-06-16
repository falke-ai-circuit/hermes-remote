# ROADMAP — hermes-remote v0.1.0-a0

## Phase Overview

| Phase | Scope | Agents | Deliverable | Status |
|-------|-------|--------|-------------|--------|
| **A** | Blueprint + Roadmap | Orchestrator | BLUEPRINT.md, ROADMAP.md | ✅ Complete |
| **B** | Parallel Coding | 4 coder subagents | Protocol, Agent, Server, Plugin | ✅ Complete |
| **C** | Review + Compile + Fixes | Reviewer + Orchestrator | 8 bugs fixed, binary compiles, all 4 endpoints verified | ✅ Complete |
| **D** | Integration Test | Operative (GWVXG74) | Agent connects from remote host → server, tools work | ⏳ Pending |
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

## Phase D — Integration Test (GWVXG74) ⏳

| Test | Procedure | Pass Criteria |
|------|-----------|---------------|
| **D1** | Cross-compile for target arch | Binary builds clean |
| **D2** | Transfer binary to GWVXG74 | falke-remote file send |
| **D3** | Start server on GWVXG74 | `./server --addr :7700` binds |
| **D4** | Start agent in silent mode | Agent appears in registry |
| **D5** | remote_shell test | `remote_shell agent="gwvxg74" command="uname -a"` returns Linux |
| **D6** | remote_fs_read test | `remote_fs_read agent="gwvxg74" path="/etc/hostname"` returns hostname |
| **D7** | remote_screenshot test | Screenshot captured and returned |
| **D8** | Interactive mode | CLI prompt appears, LLM responds, tool executes on remote |

## Phase E — Production Hardening ⏳

- TLS mutual authentication (client certs)
- Token rotation (expiring tokens, refresh flow)
- Automatic reconnect on disconnect
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
| D | 1 turn | ⏳ Pending — GWVXG74 access needed |
| E | 1-2 turns | ⏳ Pending |
| F | 1 turn | ⏳ Pending |

**v0.1.0-a0 delivered: 3 commits, 2 binaries, 5 remote tools, 8 bugs fixed. Ready for Phase D integration test.**
