# hermes-remote

Remote agent for the Hermes ecosystem. Run Hermes natively on any remote machine using the main server's LLM infrastructure.

## Quick Start

```bash
# Build
make build

# Start server (on main Hermes host)
./cmd/server/server --addr :7700 --token "hermes.circuit.remote.2026"

# Create a config file on the remote machine (hermes-remote.json):
cat > hermes-remote.json << 'EOF'
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
./cmd/hermes-remote/hermes-remote --config hermes-remote.json

# Or use the default config path (hermes-remote.json in the current directory)
./cmd/hermes-remote/hermes-remote
```

## Usage

```
Hermes Remote Assistant v0.1.0
A remote assistant tool for the Hermes AI ecosystem

Usage:
  HermesRemote.exe [--config hermes-remote.json]
```

All connection settings are read from a JSON config file. Run with `--help` to see
all available config fields and an example config.

### Config File Fields

- **server** — WebSocket server URL (`ws://` or `wss://`)
- **token** — Authentication token
- **name** — Display name for this agent (default: `hermes-remote`)
- **mode** — `silent` (daemon) or `interactive` (CLI prompt) (default: `silent`)
- **listen** — Address for inbound connections (e.g. `:7700`)
- **maxRetries** — Max reconnect attempts; `0` = infinite (default: `0`)
- **backoffMin** — Min reconnect backoff (default: `1s`)
- **backoffMax** — Max reconnect backoff (default: `60s`)
- **tokenFile** — Path to persist rotated token (default: `.hermes-remote-token`)
- **cert** — CA certificate (PEM) for verifying server TLS on `wss://`
- **clientCert** — Client certificate (PEM) for mTLS
- **clientKey** — Client key (PEM) for mTLS
- **certFile** — TLS certificate (PEM) for inbound server mode
- **keyFile** — TLS key (PEM) for inbound server mode

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

## License

MIT