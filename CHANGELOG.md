# PROBE — Changelog

All notable changes to PROBE are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/), versioning follows [Semantic Versioning](https://semver.org/).

## [v1.2.0] — 2026-07-23

### Changed
- Complete CSS rewrite with cyberpunk matrix green theme
- JetBrains Mono + Share Tech Mono fonts imported from Google Fonts
- All icons replaced with Lucide React (proper vector icons, no more emoji/Unicode)
- Sidebar: Radar logo icon, nav icons with glow on hover/active
- Agent tabs: Terminal, FolderTree, Cpu, ArrowLeftRight, Network, Bug, Monitor, ScrollText icons
- Terminal: green-on-black with text-shadow glow, proper $ prompt
- File browser: Folder/File icons from Lucide, commander dual-pane
- Processes: RefreshCw, Play, Square, Search, XCircle icons
- Tunnels: ArrowLeftRight, Plus, X icons
- MITM: Network, Play, Square, Activity, Pencil, Check, Trash2 icons
- Debug: Bug, Link2, Unlink, Cpu, MemoryStick, FileSearch icons
- Screen: Camera, Video, Square, Monitor icons
- Agents page: Settings2, ExternalLink icons for actions
- Login: Radar icon with glow, LogIn button icon
- All fonts: JetBrains Mono for monospace, Share Tech Mono for display
- CSS: stronger glow effects, gradient borders, scanline-ready dark backgrounds

## [v1.1.0] — 2026-07-23

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
- Files tab: commander-style dual-pane file browser with details panel
- Processes tab: auto-refresh (3s), filter by name/PID, kill buttons
- Tunnels tab: active tunnel cards with status dots, open/close/remove
- MITM tab: session cards with create/edit/delete, live traffic viewer
- Debug tab: load executable & auto-attach, module list, memory hex dump reader
- Screen tab: screenshot capture + streaming mode (2s interval)
- PROBE logo with green glow on sidebar and login page
- `--version` flag on probe-server
- Server version printed on startup log
- CONTRIBUTING.md — repo ruleset, versioning, commit conventions, architect delegation workflow

### Changed
- WebUI CSS rewritten with matrix green (#00ff41) glow theme
- Agent detail page restructured — connection info moved from top card to bottom bar
- Sidebar icons now have green glow on hover and active state
- Login page updated with PROBE branding and subtitle
- Server version constant: v1.1.0
- Client version constant: v1.1.0

### Fixed
- `v1CheckAuth` checked server connection token before operator token — operator login tokens were rejected with 401 when `--require-api-auth` was enabled. Fixed: operator auth checked first, server token as fallback.
- POST endpoints (capture, proc-list, mitm-stop, mitm-traffic, debug-detach, debug-modules, vt-scan) sent empty body causing "invalid JSON" errors. Fixed: all parameterless POSTs send `{}`.
- Screen capture parsing: API returns `{data: {data: "base64...", format: "jpeg"}}` but frontend looked for `image`/`base64`/`screenshot` fields. Fixed: `data.data` field parsed with `format` for MIME type.
- `/download/` endpoint only accepted server connection token, rejected operator tokens. Fixed: `checkAPIAuth` now checks operator bearer tokens too.
- Agent update download URL didn't include auth token — agent got 401 when downloading. Fixed: download URL includes `?token=` query parameter.
- Dashboard agent click used `window.location.href` (full page reload) instead of SPA navigation. Fixed: uses hash navigation.

## [v1.0.0] — 2026-07-23

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
- Port scan, net connections, sysinfo, file search capabilities
- 120+ unit tests with race detector
- 0/72 VirusTotal clean agent binary
- OpenAPI 3.0 spec at `/openapi.json`
- systemd service support with auto-restart