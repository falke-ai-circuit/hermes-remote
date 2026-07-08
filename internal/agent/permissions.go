package agent

import (
	"path/filepath"
	"strings"

	"github.com/falke-ai-circuit/hermes-remote/internal/protocol"
)

// Permission tier constants
const (
	PermSandboxed = "sandboxed"
	PermReadOnly  = "read-only"
	PermStandard  = "standard"
	PermFull      = "full"
)

// destructiveExecPatterns are command patterns blocked in sandboxed/standard mode.
// These commands can modify files outside the sandbox, kill processes, or
// otherwise affect the production system.
var destructiveExecPatterns = []string{
	// File deletion
	"del ", "rm ", "rmdir ", "rd ", "erase ", "remove-item",
	// Disk operations
	"format ", "diskpart", "chkdsk",
	// Process control (protect running production processes)
	"taskkill", "stop-process", "kill ",
	// System shutdown/restart
	"shutdown", "restart-computer",
	// File modification outside sandbox
	"copy ", "move ", "xcopy ", "robocopy", "rename ", "move-item", "copy-item",
	"set-content", "add-content", "clear-content", "clear-item",
	"new-item", "out-file",
	// Network operations that could download/execute arbitrary code
	"invoke-webrequest", "invoke-restmethod", "curl ", "wget",
	"start-process", // can start anything outside sandbox
	// Registry and scheduled tasks
	"reg add", "reg delete", "reg import",
	"schtasks /create", "schtasks /delete",
	// Service control
	"new-service", "set-service", "stop-service", "start-service",
	"restart-service",
}

// isDestructiveCommand checks if a command string matches any destructive pattern.
func isDestructiveCommand(cmd string) bool {
	lower := strings.ToLower(strings.TrimSpace(cmd))
	for _, pattern := range destructiveExecPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// isPathWithinSandbox checks if a given path is within the sandbox directory.
// If sandboxDir is empty, all paths are allowed. Paths are compared
// case-insensitively for Windows compatibility.
func isPathWithinSandbox(sandboxDir, targetPath string) bool {
	if sandboxDir == "" {
		return true
	}
	sb := filepath.Clean(sandboxDir)
	tp := filepath.Clean(targetPath)
	lowerSb := strings.ToLower(sb)
	lowerTp := strings.ToLower(tp)
	if lowerTp == lowerSb {
		return true
	}
	return strings.HasPrefix(lowerTp, lowerSb+string(filepath.Separator))
}

// isAllowed checks whether a command type is permitted under the permission tier.
// For exec commands, checks the destructive command filter.
// For filesystem commands, checks the path sandbox.
func isAllowed(permStr, sandboxDir, cmdType, command, path string) bool {

	// Full tier: everything allowed (but still check sandbox for fs ops)
	if permStr == "" || permStr == PermFull {
		if sandboxDir == "" {
			return true
		}
		// Even in full mode, if sandbox is set, check fs paths
		return isFSCommand(cmdType) == false || isPathWithinSandbox(sandboxDir, path)
	}

	switch permStr {
	case PermSandboxed:
		// Sandbox mode: restricted to startup directory
		// Allowed: fs-read, fs-list, fs-stat, fs-hash (within sandbox),
		//          file_save, fs_mkdir, fs_move (within sandbox),
		//          exec (non-destructive only), proc_start, proc_list,
		//          proc_kill/task_stop (PID-tracked only, checked in handler),
		//          health, display_info, ping/pong, token_rotate
		// Denied: file_remove, capture, input, tunnel, mitm, debug,
		//         clipboard_write, open_link, notify, agent_update
		switch cmdType {
		// Always allowed
		case protocol.TypePing, protocol.TypePong, protocol.TypeHealth,
			protocol.TypeDisplayInfo, protocol.TypeTokenRotate, protocol.TypeTokenRefresh:
			return true

		// FS read ops — allowed within sandbox
		case protocol.TypeFSRead, protocol.TypeFSList, protocol.TypeFSStat, protocol.TypeFSHash:
			return isPathWithinSandbox(sandboxDir, path)

		// FS write ops — allowed within sandbox
		case protocol.TypeFileSave, protocol.TypeFSMkdir, protocol.TypeFSMove:
			return isPathWithinSandbox(sandboxDir, path)

		// FS delete — DENIED in sandboxed mode
		case protocol.TypeFileRemove:
			return false

		// Exec — allowed but destructive commands blocked
		case protocol.TypeExec, protocol.TypeExecPTY:
			return !isDestructiveCommand(command)

		// Process listing — allowed (read-only)
		case protocol.TypeProcList, protocol.TypeTaskList:
			return true

		// Process start — allowed (PID tracked for later kill)
		case protocol.TypeProcStart:
			return true

		// Process kill / task stop — allowed but PID-tracked check
		// happens in the handler itself
		case protocol.TypeProcKill, protocol.TypeTaskStop:
			return true

		// Clipboard read — allowed
		case protocol.TypeClipboardRead:
			return true

		// Everything else — denied
		default:
			return false
		}

	case PermStandard:
		// Standard mode: same as sandboxed but no auto-sandbox
		// (sandbox_dir from config still applies if set)
		switch cmdType {
		case protocol.TypePing, protocol.TypePong, protocol.TypeHealth,
			protocol.TypeDisplayInfo, protocol.TypeTokenRotate, protocol.TypeTokenRefresh:
			return true

		case protocol.TypeFSRead, protocol.TypeFSList, protocol.TypeFSStat, protocol.TypeFSHash:
			return isPathWithinSandbox(sandboxDir, path)

		case protocol.TypeFileSave, protocol.TypeFSMkdir, protocol.TypeFSMove:
			return isPathWithinSandbox(sandboxDir, path)

		case protocol.TypeFileRemove:
			return false

		case protocol.TypeExec, protocol.TypeExecPTY:
			return !isDestructiveCommand(command)

		case protocol.TypeProcList, protocol.TypeTaskList:
			return true

		case protocol.TypeProcStart:
			return true

		case protocol.TypeProcKill, protocol.TypeTaskStop:
			return true

		case protocol.TypeClipboardRead:
			return true

		default:
			return false
		}

	case PermReadOnly:
		// Read-only: read files + safe exec only
		switch cmdType {
		case protocol.TypePing, protocol.TypePong, protocol.TypeHealth,
			protocol.TypeDisplayInfo, protocol.TypeTokenRotate, protocol.TypeTokenRefresh:
			return true

		case protocol.TypeFSRead, protocol.TypeFSList, protocol.TypeFSStat, protocol.TypeFSHash:
			return isPathWithinSandbox(sandboxDir, path)

		case protocol.TypeExec, protocol.TypeExecPTY:
			return !isDestructiveCommand(command)

		case protocol.TypeProcList, protocol.TypeTaskList:
			return true

		case protocol.TypeClipboardRead:
			return true

		default:
			return false
		}

	default:
		// Unknown tier → full access (backward compatible)
		return true
	}
}

// isFSCommand returns true if the command type is a filesystem operation
// that takes a path parameter.
func isFSCommand(cmdType string) bool {
	switch cmdType {
	case protocol.TypeFSRead, protocol.TypeFSList, protocol.TypeFSStat,
		protocol.TypeFSHash, protocol.TypeFileSave, protocol.TypeFileRemove,
		protocol.TypeFSMkdir, protocol.TypeFSMove:
		return true
	default:
		return false
	}
}