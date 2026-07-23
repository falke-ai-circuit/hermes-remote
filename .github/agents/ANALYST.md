# ANALYST.md — Agent Brief for PROBE

## Role
Codebase analyst. Deep-dive into existing code, protocols, and dependencies.

## When to Use
- Before any refactor or feature addition touching >2 files
- When protocol changes are proposed
- When a new platform implementation is needed

## Task Template

```
LANE: <lane-id>
ROLE: analyst
TOOLS: read_file, search_files, terminal

TASK: Analyze <component> in PROBE.

CRITICAL QUESTIONS (answer with evidence — file paths + line numbers):
1. What is the current implementation? (files, functions, data flow)
2. What are the dependencies? (internal packages, external libs)
3. What are the edge cases? (error paths, platform differences)
4. What would break if we changed X?
5. What test coverage exists?

OUTPUT: <path>/analysis-<component>.md
EVIDENCE: Every claim references a specific file + line number
```
