# CLAUDE.md — hermes-remote

This is the hermes-remote project — a remote agent for the Hermes ecosystem.

## What It Is

A single Go binary (`hermes-remote`) that runs Hermes natively on remote machines, using the main server's LLM infrastructure. No API keys on the remote. No SSH tunnels. Just Hermes, running wherever you put it.

## How to Build

```bash
make build          # Build both binaries
make cross          # Cross-compile for all platforms
```

Or manually:
```bash
go build ./cmd/hermes-remote/    # Agent binary
go build ./cmd/server/           # Server binary
```

## How to Run

```bash
# Server (on main machine)
./server --addr :7700 --token "hermes.circuit.remote.2026"

# Agent — silent mode (daemon, controlled via operative profile)
./hermes-remote --connect wss://server:7700 --token "..." --mode silent

# Agent — interactive mode (full Hermes CLI on remote)
./hermes-remote --connect wss://server:7700 --token "..." --mode interactive

# Agent — dual mode (daemon + accepts inbound connections)
./hermes-remote --listen :7700 --token "..."
```

## Architecture

```
Server (LLM proxy + session manager) ← WebSocket → Agent (runs on remote machine)
```

The agent runs a full Hermes agent loop (system prompt → LLM call → tool dispatch → response) using the server's credentials. Tools execute locally on the remote machine.

## Project Structure

```
hermes-remote/
├── cmd/
│   ├── hermes-remote/    # Agent binary (CLI entry point)
│   └── server/           # Server binary
├── internal/
│   ├── agent/            # Agent loop, connection management, command dispatch
│   ├── platform/         # Platform interface + Linux implementation
│   ├── protocol/         # WebSocket protocol, message types, TLS, binary frames
│   └── server/           # Multi-session server, agent registry, LLM proxy
├── tool/
│   ├── plugin.py         # Hermes plugin (5 remote tools)
│   └── plugin.yaml       # Plugin manifest
├── .github/
│   ├── workflows/        # CI (build.yml) + release (release.yml)
│   └── agents/           # Agent briefs (ANALYST, ARCHITECT, CODER, REVIEWER, OPERATIVE)
├── AGENTS.md             # Agent delegation rules
├── CLAUDE.md             # This file
├── project_knowledge.json # Hot cache + architecture map + gotchas
├── BLUEPRINT.md          # Operational blueprint
├── ROADMAP.md            # Phase overview + timeline
├── CHANGELOG.md          # Release history
├── CONTRIBUTING.md       # PR process + conventions
├── README.md             # Project overview
├── LICENSE               # MIT
├── Makefile              # Build, test, cross-compile
└── .gitignore
```

## Key Facts

- **Module path:** `github.com/falke-ai-circuit/hermes-remote`
- **Go version:** 1.22.5 (at `/opt/data/go/bin/go`)
- **Single dependency:** `gorilla/websocket v1.5.3`
- **Protocol:** WebSocket + JSON envelope, 25 command types, 9 error codes
- **Plugin tools:** `remote_agent_list`, `remote_shell`, `remote_fs_read`, `remote_fs_write`, `remote_screenshot`
- **Current version:** `v0.1.0-a0` (3 commits, 8 bugs fixed)
- **Next phase:** Phase D — integration test on GWVXG74
