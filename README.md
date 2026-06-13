# hermes-remote

Remote agent for the Hermes ecosystem. Run Hermes natively on any remote machine using the main server's LLM infrastructure.

## Quick Start

```bash
# Build
go build ./cmd/hermes-remote/go build ./cmd/server/

# Start server (on main Hermes host)
./server --addr :7700 --token "hermes.circuit.remote.2026"

# Connect agent (on remote machine) — silent mode
./hermes-remote --connect wss://server:7700 --token "hermes.circuit.remote.2026" --mode silent

# Connect agent — interactive mode (full Hermes CLI)
./hermes-remote --connect wss://server:7700 --token "hermes.circuit.remote.2026" --mode interactive
```

## Two Modes

| Mode | Command | Behavior |
|------|---------|----------|
| **Silent** | `--mode silent` | Daemon. Controlled from main server via operative profile tools. |
| **Interactive** | `--mode interactive` | Full Hermes CLI session. LLM runs on server, tools run on remote. |

## Operative Tools

Once the plugin is installed, the operative profile gets 5 new tools:

| Tool | What it does |
|------|-------------|
| `remote_agent_list` | List all connected agents with health |
| `remote_shell` | Execute shell command on agent |
| `remote_fs_read` | Read file from agent filesystem |
| `remote_fs_write` | Write file to agent filesystem |
| `remote_screenshot` | Capture screen from agent |

## Architecture

```
Server (LLM proxy + session manager) ← WebSocket → Agent (runs full Hermes loop on remote)
```

Remote machines never get API keys. LLM inference happens on the server. Tools (terminal, file, screen, input) execute locally on the remote machine.

## Build

```bash
make build          # Linux amd64
make cross          # All platforms (linux/amd64, linux/arm64, windows/amd64, darwin/amd64, darwin/arm64)
make vet            # Run go vet
make test           # Run tests
```

## License

MIT
