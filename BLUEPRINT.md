# BLUEPRINT — hermes-remote v0.1 (a0)

**Author:** Architect (via Orchestrator)
**Date:** 2026-06-13
**Status:** DRAFT → approved by GR15
**Repo:** `github.com/falke-ai-circuit/hermes-remote`

---

## 1. Problem

The operator agent needs to control remote machines — desktops, laptops, servers, phones. The existing falke-remote relay (CT101:7700) is a Windows-only, central-relay, HTTP REST system with no shell access. We need a **single binary** that runs Hermes natively on any remote machine, using the main server's LLM infrastructure.

## 2. Architecture Decision

**hermes-remote binary = Hermes agent running on remote machine, with LLM calls routed through the main server.** No API keys on the remote. No SSH tunnels. No raw shell relay. Just Hermes, running wherever you put it.

```
┌─────────────────────────────────────────┐
│  MAIN SERVER                            │
│  ┌───────────────────────────────┐      │
│  │ hermes-remote server :7700    │      │
│  │ (WebSocket relay + LLM proxy  │      │
│  │  + session manager)           │      │
│  └───────────────────────────────┘      │
└──────────────────┬──────────────────────┘
        │              │              │
   ┌────▼─────┐  ┌───▼──────┐  ┌───▼──────┐
   │ Silent    │  │ Silent   │  │ Interactive│
   │ daemon    │  │ daemon   │  │ CLI prompt │
   └───────────┘  └──────────┘  └───────────┘
```

## 3. Two Modes

| Mode | Command | Behavior |
|------|---------|----------|
| **Silent** | `--mode silent` | Daemon in background. Visible as local instance to server. Controlled via operative profile. |
| **Interactive** | `--mode interactive` | Full Hermes CLI session. Real prompt, tools, memory. LLM runs on server. Tools on remote. |
| **Dual** | `--listen :7700` | Both modes simultaneously — daemon + can accept inbound connections. |

## 4. Protocol

- **Transport:** WebSocket (RFC 6455) over TLS 1.3
- **Auth:** Token in header `Authorization: Bearer <token>`
- **Messages:** JSON envelope with `{id, type, params, result, error}`
- **Commands:** 25 commands across 5 categories (shell, filesystem, screen, input, system)
- **Heartbeat:** 15s ping/pong, 3-miss disconnect threshold

## 5. Server Components

| Component | Purpose |
|-----------|---------|
| **LLM Proxy** | Routes LLM calls to providers (DeepSeek, MiniMax, Ollama) using server's API keys |
| **Session Manager** | Creates one Hermes session per connected agent — memory, skills, context |
| **Agent Registry** | Persisted JSON registry of all connected agents with health |
| **WS Relay** | WebSocket server on :7700, handles connect/auth/message routing |

## 6. Agent Components

| Component | Purpose |
|-----------|---------|
| **Agent Loop** | Full Hermes agent loop (system prompt → LLM call → tool dispatch → response) |
| **Platform Adapters** | Linux (native), Windows (stub), macOS (stub) — for platform-specific tools |
| **Protocol Client** | WebSocket dial, ping/pong, message serialization |

## 7. Plugin Integration

Operative profile gets new tools via `kind: standalone` Hermes plugin:
- `remote_agent_list` — list all connected agents
- `remote_shell` — execute command on agent
- `remote_fs_read` — read file from agent
- `remote_fs_write` — write file to agent
- `remote_screenshot` — capture screen from agent

## 8. File Structure

```
hermes-remote/
├── cmd/
│   └── hermes-remote/
│       └── main.go              # CLI flags, mode selection
├── internal/
│   ├── protocol/
│   │   ├── messages.go          # All message types
│   │   ├── websocket.go         # Dial/Listen/Upgrade
│   │   ├── binary.go            # Binary frame encoding
│   │   └── server.go            # Server wrapper
│   ├── server/
│   │   ├── server.go            # Multi-session WS server
│   │   ├── registry.go          # Agent registry (persisted JSON)
│   │   ├── proxy.go             # LLM proxy to providers
│   │   └── session.go           # Per-agent Hermes session
│   ├── agent/
│   │   ├── agent.go             # Agent loop + command dispatch
│   │   └── handlers.go           # Command handlers (shell, fs, etc.)
│   └── platform/
│       ├── platform.go          # Platform interface
│       ├── platform_linux.go    # Linux implementation
│       └── platform_windows.go  # Windows stub
├── tool/
│   └── plugin.py                # Hermes plugin registration
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## 9. Success Criteria

| # | Criterion | Evidence |
|---|-----------|----------|
| 1 | Binary compiles for linux/amd64 | `go build -o hermes-remote ./cmd/hermes-remote` exits 0 |
| 2 | Server starts on :7700 with TLS | `./hermes-remote server --port 7700` accepts connections |
| 3 | Agent connects in silent mode | `./hermes-remote --connect wss://localhost:7700 --mode silent` registers in registry |
| 4 | Agent connects in interactive mode | `./hermes-remote --connect wss://localhost:7700 --mode interactive` opens CLI |
| 5 | Operative tools work | `remote_agent_list` shows connected agents |
| 6 | Remote shell works | `remote_shell agent="a0-test" command="echo hello"` returns `hello` |
| 7 | Kali Linux test | Binary compiled and deployed, connects from Kali container to server |
| 8 | Multi-agent | 3 agents connected simultaneously, all visible in registry |
