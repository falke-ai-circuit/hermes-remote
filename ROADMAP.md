# ROADMAP — hermes-remote v0.1 (a0)

## Phase Overview

| Phase | Scope | Agents | Deliverable |
|-------|-------|--------|-------------|
| **A** | Blueprint + Roadmap | Orchestrator (done) | BLUEPRINT.md, ROADMAP.md |
| **B** | Parallel Coding | 4 coder subagents | Protocol, Agent, Server, Plugin |
| **C** | Review + Compile | Reviewer + Orchestrator | Binary compiles, tests pass, syntax clean |
| **D** | Integration Test | Operative (Kali) | Agent connects from Kali → server, tools work |
| **E** | Push to GitHub | Orchestrator | Repo on `falke-ai-circuit/hermes-remote` |

---

## Phase B — Parallel Coding (4 parallel subagents)

### B1: Protocol Layer (coder)
- **Goal:** Implement `internal/protocol/` — messages, websocket, binary frames, server wrapper
- **Files:** messages.go (all 25+ message types), websocket.go (dial/listen/upgrade), binary.go, server.go
- **Must:** Compile independently with `go build ./internal/protocol/...`
- **Depends on:** Nothing (first tier)

### B2: Server Layer (coder)
- **Goal:** Implement `internal/server/` — multi-session WS server, registry, LLM proxy, session manager
- **Files:** server.go, registry.go (persisted JSON), proxy.go (routes to providers), session.go (per-agent Hermes session)
- **Must:** Compile, accept connections, register agents
- **Depends on:** B1 (protocol)

### B3: Agent Layer + CLI (coder)
- **Goal:** Implement `internal/agent/`, `internal/platform/`, `cmd/hermes-remote/main.go`
- **Files:** agent.go (loop + dispatch), handlers.go (25 commands), platform_linux.go, main.go (CLI flags)
- **Must:** Compile to binary, accept --mode silent|interactive
- **Depends on:** B1, B2

### B4: Plugin + Tools (coder)
- **Goal:** Implement `tool/plugin.py` — Hermes plugin registration for operative profile
- **Files:** plugin.py (register tools: remote_agent_list, remote_shell, remote_fs_read, remote_fs_write, remote_screenshot), register plugin manifest
- **Must:** Plugin loads, tools appear in operative profile
- **Depends on:** B1 (message types)

---

## Phase C — Review + Compile

| Check | Who | Evidence |
|-------|-----|----------|
| B1 compiles | Reviewer | `go build ./internal/protocol/...` exits 0 |
| B2 compiles | Reviewer | `go build ./internal/server/...` exits 0 |
| B3 compiles | Reviewer | `go build ./cmd/hermes-remote/` exits 0 |
| Syntax check | Orchestrator | All .go files pass `go vet` |
| Cross-module | Orchestrator | `go build ./...` exits 0 |
| Server start | Orchestrator | `./hermes-remote server --port 7700` binds |
| Agent connect | Orchestrator | `./hermes-remote --connect wss://localhost:7700 --mode silent` works |

---

## Phase D — Integration Test (Kali container)

| Test | Procedure | Pass Criteria |
|------|-----------|---------------|
| **D1** | Run `./hermes-remote --connect wss://host:7700 --mode silent` on Kali | Agent appears in registry |
| **D2** | `remote_shell agent="a0-kali" command="uname -a"` | Returns Kali Linux kernel |
| **D3** | `remote_fs_read agent="a0-kali" path="/etc/hostname"` | Returns "kali" |
| **D4** | Start interactive mode `--mode interactive` on Kali | CLI prompt appears |
| **D5** | Issue command in interactive session | LLM responds, tool executes on Kali |

---

## Phase E — GitHub Push

```
git init
git remote add origin git@github.com:falke-ai-circuit/hermes-remote.git
git add -A
git commit -m "feat: hermes-remote v0.1 (a0) — remote agent for Hermes ecosystem"
git push -u origin main
```

---

## Timeline

| Phase | Est. Time | Status |
|-------|-----------|--------|
| A | Done | ✅ Complete |
| B | 1-2 turns (parallel) | → Next |
| C | 1 turn | Pending |
| D | 1 turn | Pending |
| E | 1 turn | Pending |

**Total: ~4 turns to working remote agent with validated connectivity.**
