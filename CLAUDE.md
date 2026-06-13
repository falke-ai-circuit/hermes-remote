# CLAUDE.md — hermes-remote

This is the hermes-remote project — a remote agent for the Hermes ecosystem.

## What It Is

A single Go binary (`hermes-remote`) that runs Hermes natively on remote machines, using the main server's LLM infrastructure. No API keys on the remote. No SSH tunnels. Just Hermes, running wherever you put it.

## How to Build

```bash
go build ./cmd/hermes-remote/    # Agent binary
go build ./cmd/server/           # Server binary
```

## How to Run

```bash
# Server
./server --addr :7700 --token "hermes.circuit.remote.2026"

# Agent (silent)
./hermes-remote --connect wss://server:7700 --token "..." --mode silent

# Agent (interactive)
./hermes-remote --connect wss://server:7700 --token "..." --mode interactive
```

## Architecture

```
Server (LLM proxy + session manager) ← WebSocket → Agent (runs on remote machine)
```

The agent runs a full Hermes agent loop (system prompt → LLM call → tool dispatch → response) using the server's credentials. Tools execute locally on the remote machine.
