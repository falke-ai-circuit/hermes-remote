# OPERATIVE.md — Agent Brief for PROBE

## Role
Infrastructure operator. Deploy, monitor, and troubleshoot PROBE on target machines.

## When to Use
- Deploying PROBE to a new remote host
- Troubleshooting connection issues
- Verifying platform tools work on target OS
- Cross-compiling for target architecture

## Task Template

```
LANE: <lane-id>
ROLE: operative
TOOLS: terminal, read_file, search_files

TASK: <deploy/test/troubleshoot> PROBE on <target>.

DEPLOYMENT:
1. Cross-compile: make cross (or GOOS=<os> GOARCH=<arch> go build)
2. Transfer binary: scp/falke-remote to target
3. Start server: ./server --addr :7700 --token "<token>"
4. Start agent: ./probe-client --connect wss://<host>:7700 --token "<token>" --mode silent
5. Verify: agent appears in server registry, tools respond

TROUBLESHOOTING:
- Connection refused → check server is running, port is open
- TLS error → check cert fingerprint, try --insecure for testing
- Tool timeout → check platform deps (xdotool, import/scrot, xclip)
- Agent disconnect → check network stability, token validity

PLATFORM VERIFICATION:
- Linux: xdotool, import or scrot, xclip installed
- macOS: osascript (screenshot), pbcopy/pbpaste (clipboard)
- Windows: PowerShell, .NET screen capture

EVIDENCE:
- Binary transfer confirmation (file size + hash)
- Server startup log
- Agent connection log
- Tool roundtrip test output
```
