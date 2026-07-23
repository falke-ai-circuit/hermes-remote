# PROBE — Changelog

All notable changes to PROBE are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/), versioning follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- Agent capabilities toggle UI on Agents page — toggle on/off per agent with redeploy
- `GET /api/v1/agents/{id}/capabilities` — returns agent's current capabilities
- `POST /api/v1/agents/{id}/redeploy` — rebuild agent with new capabilities, push update through existing connection
- VirusTotal scan integration — `internal/server/virustotal.go` with v3 API client
- `POST /api/v1/builds/{id}/vt-scan` — trigger VT scan on completed build
- `GET /api/v1/builds/{id}/vt-scan` — get current VT scan status
- `--vt-api-key` flag + `PROBE_VT_API_KEY` env var for VT configuration
- Auto VT scan after build completion (when API key configured)
- VT scan status badges in Builder page (clean/dirty/scanning/not scanned)
- Matrix green glow theme — all UI elements use #00ff41 with glow effects
- Agent detail page redesigned: tabs primary, connection info in bottom bar
- Terminal tab: interactive shell with command history (↑↓), Ctrl+L clear
- Files tab: commander-style dual-pane file browser
- Processes tab: auto-refresh (3s), filter by name/PID, kill buttons
- Tunnels tab: active tunnel cards with status dots
- MITM tab: session cards with create/edit/delete, live traffic viewer
- Debug tab: load executable & auto-attach, module list, memory hex dump
- Screen tab: screenshot capture + streaming mode (2s interval)
- PROBE logo with green glow on sidebar and login page
- CONTRIBUTING.md — repo ruleset, versioning, commit conventions

### Changed
- WebUI CSS rewritten with matrix green glow theme
- Agent detail page restructured — connection info moved from top card to bottom bar
- Sidebar icons now have green glow on hover and active state
- Login page updated with PROBE branding and subtitle

### Fixed
- Tailscale IP filter blocking legitimate browser API access — disabled by default (0.0.0.0/0)
- Dashboard agent click now uses hash navigation instead of full page reload

## [1.0.0] — 2026-07-23

### Added
- PROBE server — Go backend with REST API v1, WebSocket agent protocol
- PROBE client — cross-platform agent with capability-driven architecture
- RBAC with operators and roles (admin, operator, viewer)
- Audit logging for all agent actions
- Agent builder with cross-compilation, PE disguise, build profiles
- Task scheduler (delayed, recurring, offline queue)
- React WebUI (Vite + TypeScript) embedded in server binary
- Resumable chunked file transfers with SHA256 verification
- Agent self-update mechanism (download → verify → swap → kill old)
- Tunnels, MITM proxy, debug (attach/memory/modules), screen capture
- 120+ unit tests with race detector
- 0/72 VirusTotal clean agent binary