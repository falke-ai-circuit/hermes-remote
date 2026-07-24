# Go AST Obfuscation + Evasion Tool

Custom source-level obfuscation and evasion tool for Go binaries. Combines
XOR string encryption with optional anti-debug, beacon jitter, and API
hashing techniques. Avoids known-tool signatures (garble, gobfuscate) by
using unique patterns not in any threat intelligence database.

## Usage

```bash
# Build the tool
go build -o obfuscate ./tools/obfuscate/

# Apply XOR string encryption only (default, same as before)
cp -r /path/to/probe /tmp/probe-obf
./obfuscate /tmp/probe-obf

# Apply ALL evasion techniques (recommended)
cp -r /path/to/probe /tmp/probe-obf
./obfuscate -all /tmp/probe-obf

# Apply individual techniques
./obfuscate -jitter /tmp/probe-obf       # Beacon jitter only
./obfuscate -antidebug /tmp/probe-obf    # Anti-debug + VM evasion only
./obfuscate -apihash /tmp/probe-obf      # API hashing only

# Build the obfuscated binary
GOTOOLCHAIN=local GOOS=windows GOARCH=amd64 go1.23.12 build -trimpath \
  -o ./build/ProbeClient.exe ./cmd/probe-client/
```

## Flags

| Flag | Description | VT Impact |
|------|-------------|-----------|
| (none) | XOR string encryption only (default) | 0/69 baseline |
| `-jitter` | Add random 0-25% jitter to all `time.Sleep` calls | AV-neutral, defeats IDS beacon detection |
| `-antidebug` | Generate evasion package with debugger/VM/sandbox checks | Drops 1 detection (Bkav), defeats automated sandboxes |
| `-apihash` | Hash Windows API names (syscall.NewLazyDLL/NewProc) | Drops 1 detection (Bkav), hides intent from RE |
| `-all` | Apply all techniques | Best combined result |

## Execution Order

When `-all` is used, phases run in this order:

1. **Anti-Debug** — generates `internal/evasion/` package with `init()` that
   checks for debuggers, VMs, sandboxes. Injects blank import into `main.go`.
2. **Jitter** — transforms `time.Sleep(X)` → `_jsleep(X)` which adds random
   0-25% jitter. Generates `zjitter.go` helper per package.
3. **API Hashing** — transforms `syscall.NewLazyDLL("name")` →
   `_resolveDLLByName(_hName)`. Generates `zapihash.go` helper per package.
4. **XOR String Encryption** — encrypts ALL string literals ≥3 bytes with
   per-string random XOR keys. Runs LAST so it also encrypts strings in
   generated evasion/jitter/apihash files.

## What each technique does

### XOR String Encryption (always on)

Every string literal ≥3 bytes is replaced with `_d([]byte{0x..., 0x...}, 0xKEY)`.
Each string gets a unique random key (1-255). `const` blocks containing strings
are converted to `var`. A `zdecode.go` file is generated per package.

### Beacon Jitter (`-jitter`)

Replaces `time.Sleep(X)` with `_jsleep(X)` which adds `rand(0, X/4)` jitter.
Defeats network IDS beacon detection (RITA, Zeek, Suricata) that fingerprints
regular heartbeat intervals. Zero AV impact — invisible to static scanners.

### Anti-Debug + VM/Sandbox Evasion (`-antidebug`)

Generates `internal/evasion/` package with `init()` that runs before `main()`:
- `IsDebuggerPresent` + `CheckRemoteDebuggerPresent` (Windows)
- VM checks: CPU count <2, RAM <2GB, uptime <10min
- Timing check: sleep 2s, detect accelerated sleep (sandbox)
- VMware/VBox/Hyper-V/QEMU MAC prefix detection
- Sandbox environment variables (SANDBOX, VBOX, CUCKOO, etc.)
- Sandbox usernames (sandbox, malware, cuckoo, user)

If any check fails → `os.Exit(0)` (silent exit, no crash).

VT result: drops 1 detection (Bkav). The Windows diagnostic API patterns
shift the ML profile toward "system utility."

### API Hashing (`-apihash`)

Replaces `syscall.NewLazyDLL("name")` with `_resolveDLLByName(_hName)` where
`_hName` is a SHA-256 hash constant. API names are stored as hash values
(uint64) — no plaintext API names in the binary's strings table.

VT result: drops 1 detection (Bkav). Hash constants look like random numbers
to static analysis.

## What it skips

- Import paths (would break compilation)
- Struct tags (`json:"field"`)
- Raw string literals (backtick strings)
- Strings <3 bytes or >500 bytes
- Test files (`*_test.go`)
- Server-only files (`cmd/probe-server/`)
- Generated files (`zdecode.go`, `zjitter.go`, `zapihash.go`)
- The evasion package is skipped by API hashing (XOR handles its strings)

## What NOT to do (tested on VT, 2026-07-23)

- **uTLS/JA3 spoofing** — triggers CrowdStrike grayware. Skip.
- **garble** — 15-16/70 detections. 15 engines flag it.
- **UPX packing** — 9/70 detections. Known packer signature.
- **Stripping (`-s -w`)** — triggers Microsoft Wacatac.B!ml.
- **Dead code insertion** — triggers Kaspersky.
- **Self-signed certs** — re-introduces Microsoft detection.

## Test results

- **XOR obf + -trimpath + Go 1.23.12**: 0/66 VT detections (2026-07-23)
- **+ jitter**: 0/66 (AV-neutral, operational benefit only)
- **+ anti-debug**: drops 1 detection (Bkav)
- **+ API hashing**: drops 1 detection (Bkav)
- **Recommended recipe**: `obfuscate -all` + `-trimpath` + Go 1.23.12 + zero ldflags