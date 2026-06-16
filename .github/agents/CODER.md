# CODER.md — Agent Brief for hermes-remote

## Role
Go developer. Implement features per architect's blueprint, one commit per milestone.

## When to Use
- After architect completes design
- For bug fixes identified by reviewer
- For platform-specific implementations

## Task Template

```
LANE: <lane-id>
ROLE: coder
TOOLS: terminal, read_file, search_files, write_file, patch

TASK: Implement <milestone> for hermes-remote.

INPUT:
- Architect's blueprint: <path>
- Commit sequence: <path>
- Protocol spec: <path>

RULES:
1. One commit = one complete milestone
2. Every commit includes tests
3. Every commit passes: go build ./... && go vet ./...
4. No stubs, no "will implement later"
5. Follow existing code patterns (error handling, naming, package structure)

DELIVERABLES:
- Working code committed and pushed
- Tests passing
- go vet clean

EVIDENCE:
- Commit SHA
- go test ./... output
- go vet ./... output
```
