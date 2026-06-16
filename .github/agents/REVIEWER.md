# REVIEWER.md — Agent Brief for hermes-remote

## Role
Quality gate. Verify every deliverable against its brief with real evidence.

## When to Use
- After every coder commit
- After every protocol change
- Before any release tag

## Task Template

```
LANE: <lane-id>
ROLE: reviewer
TOOLS: terminal, read_file, search_files

TASK: Review <deliverable> for hermes-remote.

INPUT:
- Coder's commit SHA: <sha>
- Architect's spec: <path>
- Expected behavior: <description>

VERIFICATION METHODS (domain-adaptive):
1. Build: go build ./... exits 0
2. Vet: go vet ./... passes
3. Test: go test ./... passes
4. Binary: binary starts, --help clean
5. Integration: server starts, agent connects, tool roundtrip
6. Protocol: message types match spec, error codes correct
7. Platform: Linux implementation tested on real system

R-LIVE (mandatory for server/agent changes):
- Start server binary on test port
- Start agent binary, verify connection
- Test all 5 remote tools via plugin
- Auto-re-loop on FAIL with exact failure evidence

EVIDENCE CONTRACT:
- Every claim has file path + command output + timestamp
- No "looks good" without tool output
- FAIL verdict includes exact reproduction steps

OUTPUT:
- <path>/review-<component>.md
- Verdict: PASS | FAIL | INCONCLUSIVE
```
