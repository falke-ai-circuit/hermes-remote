# PROBE

Remote agent for the Hermes ecosystem. Run Hermes natively on any remote machine using the main server's LLM infrastructure.

## Quick Start

```bash
# Build
make build

# Start server (on main Hermes host)
./cmd/server/server --addr :7700 --token "hermes.circuit.remote.2026"

# Create a config file on the remote machine (probe-client.json):
cat > probe-client.json << 'EOF'
{
  "server": "ws://your-server:7700",
  "token": "your-auth-token",
  "name": "my-computer",
  "mode": "silent",
  "maxRetries": 0,
  "backoffMin": "1s",
  "backoffMax": "60s"
}
EOF

# Run the agent with the config file
./cmd/probe-client/probe-client --config probe-client.json

# Or use the default config path (probe-client.json in the current directory)
./cmd/probe-client/probe-client
```

## Usage

```
PROBE Client v1.5.0
A remote assistant tool for the Hermes AI ecosystem

Usage:
  ProbeClient.exe [--config probe-client.json]
```

All connection settings are read from a JSON config file. Run with `--help` to see
all available config fields and an example config.

### Config File Fields

- **server** — WebSocket server URL (`ws://` or `wss://`)
- **token** — Authentication token
- **name** — Display name for this agent (default: `probe-client`)
- **mode** — `silent` (daemon) or `interactive` (CLI prompt) (default: `silent`)
- **listen** — Address for inbound connections (e.g. `:7700`)
- **maxRetries** — Max reconnect attempts; `0` = infinite (default: `0`)
- **backoffMin** — Min reconnect backoff (default: `1s`)
- **backoffMax** — Max reconnect backoff (default: `60s`)
- **tokenFile** — Path to persist rotated token (default: `.probe-token`)
- **cert** — CA certificate (PEM) for verifying server TLS on `wss://`
- **clientCert** — Client certificate (PEM) for mTLS
- **clientKey** — Client key (PEM) for mTLS
- **certFile** — TLS certificate (PEM) for inbound server mode
- **keyFile** — TLS key (PEM) for inbound server mode

## WebUI

The embedded React WebUI (Vite + TypeScript) provides a full management interface:

**Sidebar navigation:** Dashboard, Agents, Tasks, Transfers, Credentials, Builder, Profiles, Settings

**Pages:**
- **Dashboard** — agent overview, health status, quick actions
- **Agents** — agent list with search, capabilities toggle, redeploy; Agent Detail with breadcrumb navigation (Agents > [Name] > [Tab]) and tabs: Terminal, Files, Processes, Tunnels, MITM, Debug, Screen, Audit
- **Tasks** — scheduled task management (delayed, recurring, offline queue)
- **Transfers** — global file transfer view across all agents with progress bars, status badges, pause/resume, filter by status
- **Credentials** — scan agents for passwords, hashes, API keys, tokens, connection strings, AWS keys, private keys via regex; manual text paste scanner; OS-specific gather commands (Windows/Linux/macOS)
- **Builder** — 5-step agent build wizard with capability checkboxes (tooltips on all 9 capabilities), cross-compilation, VirusTotal scan integration
- **Profiles** — build profile management
- **Settings** — server configuration

**Builder capability tooltips:** all 9 capability checkboxes have descriptive `title` attributes explaining what each capability does.

## Two Modes

- **Silent** — `--mode silent` in config. Daemon controlled from the main server via operative profile tools.
- **Interactive** — `--mode interactive` in config. Full Hermes CLI session. LLM runs on server, tools run on remote.

## Operative Tools

Once the plugin is installed, the operative profile gets 5 new tools:

- **`remote_agent_list`** — List all connected agents with health
- **`remote_shell`** — Execute shell command on agent
- **`remote_fs_read`** — Read file from agent filesystem
- **`remote_fs_write`** — Write file to agent filesystem
- **`remote_screenshot`** — Capture screen from agent

## Architecture

```
Server (LLM proxy + session manager) ← WebSocket → Agent (runs full Hermes loop on remote)
```

Remote machines never get API keys. LLM inference happens on the server. Tools (terminal, file, screen, input) execute locally on the remote machine.

## Build

```bash
make build          # Build both binaries
make cross          # All platforms (linux/amd64, linux/arm64, windows/amd64, darwin/amd64, darwin/arm64)
make windows        # Windows exe only (with version info, stripped symbols)
make vet            # Run go vet
make test           # Run tests
```

### Build Tag Variants (v1.5.0+)

The single `cmd/probe` source tree produces three build variants via Go build tags. This allows deploying minimal-capability binaries to endpoints — server and relay code is excluded from client-only builds, reducing the reverse-engineering surface.

| Variant | Build Command | Subcommands | Binary Size |
|---------|--------------|-------------|-------------|
| **Client-only** (default) | `go build -trimpath` | `probe connect` | ~9.6 MB |
| **Server** | `go build -trimpath -tags server` | `probe serve` + `probe connect` | ~11.1 MB |
| **Relay** | `go build -trimpath -tags relay` | `probe relay` + `probe connect` | ~9.7 MB |

If a user invokes a subcommand not compiled in (e.g. `probe serve` on a client-only build), `serve_stub.go` / `relay_stub.go` print a helpful error explaining which build tag to use.

**Security benefit**: Endpoints only receive the client-only binary. Server and relay code is excluded at compile time — an adversary capturing an endpoint binary cannot reverse-engineer the server API surface or relay framing protocol. This addresses the single-binary RE exposure concern raised in Shadow review item #5. The previous v1.4.0 unified binary (all 3 modes in one binary) had 1/67 Microsoft Wacatac detection on VirusTotal; the client-only build tag variant has a smaller footprint and 0 detections.

## License

MIT