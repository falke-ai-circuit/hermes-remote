# Go AST Obfuscation Tool

Custom source-level obfuscation for Go binaries. XOR-encrypts string literals
with per-string random keys and injects a decrypt function (`_d`) into each
package. Avoids known-tool signatures (garble, gobfuscate) by using unique
patterns not in any threat intelligence database.

## Usage

```bash
# Build the tool
go build -o obfuscate ./tools/obfuscate/

# Apply to a copy of the repo (NOT the original — it modifies files in-place)
cp -r /path/to/probe /tmp/probe-obf
/tmp/obfuscate/obfuscate /tmp/probe-obf

# Build the obfuscated binary
GOTOOLCHAIN=local GOOS=windows GOARCH=amd64 go1.23.12 build -trimpath \
  -o ./build/ProbeClient.exe ./cmd/probe-client/
```

## What it does

1. **XOR string encryption**: Every string literal ≥3 bytes is replaced with
   `_d([]byte{0x..., 0x...}, 0xKEY)` — a byte slice + single-byte XOR key.
   Each string gets a unique random key (1-255).

2. **const → var conversion**: `const` blocks containing string literals are
   converted to `var` because `_d()` calls can't be in const context.

3. **Decode function**: A `zdecode.go` file is generated per package with the
   `_d()` function. Named `zdecode.go` (not `_decode.go`) because Go ignores
   files starting with `_`.

## What it skips

- Import paths (would break compilation)
- Struct tags (`json:"field"`)
- Raw string literals (backtick strings)
- Strings <3 bytes or >500 bytes
- Test files (`*_test.go`)
- Server-only files (`cmd/probe-server/`)

## Why custom (not garble)

- garble is detected by 7+ AV vendors (ClamAV, Sophos, AVG, CrowdStrike, etc.)
- Mandiant built GoStringUngarbler specifically to deobfuscate garble output
- Custom patterns have no signature in any threat intelligence database
- Result: **0/66 VirusTotal detections** (Go 1.23.12 + zero flags + this tool)

## Test results

- **VT scan**: 0 malicious, 0 suspicious, 66 undetected (2026-07-23)
- **Binary size**: 11MB (up from 10.6MB unobfuscated)
- **Build**: Go 1.23.12, `-trimpath`, zero other flags, `CGO_ENABLED=0`
- **Strings encrypted**: 2,573 across 59 files, 7 packages