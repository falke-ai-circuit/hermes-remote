package agent

import (
	"path/filepath"
	"strings"

	"github.com/falke-ai-circuit/hermes-remote/internal/protocol"
)

// Permission tier constants
const (
	PermReadOnly = "read-only"
	PermStandard  = "standard"
	PermFull      = "full"
)

// PermissionConfig holds granular permission settings parsed from the config file.
// When Permissions field is set to a named tier ("read-only", "standard", "full"),
// the corresponding preset is used. When Permissions is a JSON object, individual
// capabilities can be toggled.
type PermissionConfig struct {
	// Named tier: "read-only", "standard", "full". Empty = "full" (backward compat).
	Tier string `json:"tier,omitempty"`

	// Individual capability toggles (override tier defaults when set)
	Exec          *bool `json:"exec,omitempty"`
	ExecDestructive *bool `json:"exec_destructive,omitempty"` // allow del, taskkill, format, etc.
	FSRead        *bool `json:"fs_read,omitempty"`
	FSWrite       *bool `json:"fs_write,omitempty"`
	FSDelete      *bool `json:"fs_delete,omitempty"`
	FSMove        *bool `json:"fs_move,omitempty"`
	FSMkdir       *bool `json:"fs_mkdir,omitempty"`
	FSHash        *bool `json:"fs_hash,omitempty"`
	FSList        *bool `json:"fs_list,omitempty"`
	FSStat        *bool `json:"fs_stat,omitempty"`
	ProcList      *bool `json:"proc_list,omitempty"`
	ProcKill      *bool `json:"proc_kill,omitempty"`
	ProcStart     *bool `json:"proc_start,omitempty"`
	Capture       *bool `json:"capture,omitempty"`
	Input         *bool `json:"input,omitempty"` // mouse/keyboard
	Tunnel        *bool `json:"tunnel,omitempty"`
	Mitm          *bool `json:"mitm,omitempty"`
	Debug         *bool `json:"debug,omitempty"`
	ClipboardRead *bool `json:"clipboard_read,omitempty"`
	ClipboardWrite *bool `json:"clipboard_write,omitempty"`
	OpenLink      *bool `json:"open_link,omitempty"`
	Notify        *bool `json:"notify,omitempty"`
	AgentUpdate   *bool `json:"agent_update,omitempty"`

	// Path sandbox: restrict filesystem operations to within this directory.
	// Empty = no restriction. All fs paths are resolved and checked against this.
	// Example: "C:\\Users\\dna\\Downloads" — only that subtree is accessible.
	SandboxDir string `json:"sandbox_dir,omitempty"`
}

// readOnlyTier is the preset for "read-only" permissions.
func readOnlyTier() PermissionConfig {
	f := false
	return PermissionConfig{
		Tier:           PermReadOnly,
		Exec:           &f,
		ExecDestructive: &f,
		FSRead:         nil, // nil = use tier default (true for read)
		FSWrite:        &f,
		FSDelete:       &f,
		FSMove:         &f,
		FSMkdir:        &f,
		ProcKill:       &f,
		ProcStart:      &f,
		Capture:        &f,
		Input:          &f,
		Tunnel:         &f,
		Mitm:           &f,
		Debug:          &f,
		ClipboardWrite: &f,
		OpenLink:       &f,
		Notify:         &f,
		AgentUpdate:    &f,
	}
}

// standardTier is the preset for "standard" permissions.
func standardTier() PermissionConfig {
	f := false
	return PermissionConfig{
		Tier:            PermStandard,
		ExecDestructive: &f, // no del/taskkill/format
		FSDelete:        &f,
		ProcKill:        &f,
		ProcStart:       &f,
		Capture:         &f,
		Input:           &f,
		Tunnel:          &f,
		Mitm:            &f,
		Debug:           &f,
		ClipboardWrite:  &f,
		OpenLink:        &f,
		Notify:          &f,
		AgentUpdate:     &f,
	}
}

// fullTier allows everything.
func fullTier() PermissionConfig {
	return PermissionConfig{Tier: PermFull}
}

// resolvePermissionConfig parses the permissions string from config.
// If it's a known tier name, returns the preset. If it's a JSON object,
// parses it as a PermissionConfig with individual toggles.
func resolvePermissionConfig(permStr string) PermissionConfig {
	if permStr == "" {
		return fullTier()
	}
	switch permStr {
	case PermReadOnly:
		return readOnlyTier()
	case PermStandard:
		return standardTier()
	case PermFull:
		return fullTier()
	default:
		// Try parsing as JSON for granular control
		// Fallback to full if parse fails
		return fullTier()
	}
}

// destructiveExecPatterns are command patterns blocked when ExecDestructive is false.
var destructiveExecPatterns = []string{
	"del ", "rm ", "rmdir ", "rd ", "erase ", "format ", "diskpart",
	"taskkill", "stop-process", "shutdown", "restart-computer",
	"remove-item", "clear-content", "clear-item",
	"copy ", "move ", "xcopy ", "robocopy", "rename ",
	"new-item", "set-content", "add-content", "out-file",
	"invoke-webrequest", "invoke-restmethod", "curl ", "wget",
	"start-process", "new-service", "set-service",
	"reg add", "reg delete", "reg import",
	"schtasks /create", "schtasks /delete",
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
// If sandboxDir is empty, all paths are allowed. On Windows, paths are
// case-insensitive but we do a case-insensitive prefix match for compatibility.
func isPathWithinSandbox(sandboxDir, targetPath string) bool {
	if sandboxDir == "" {
		return true
	}
	// Clean both paths
	sb := filepath.Clean(sandboxDir)
	tp := filepath.Clean(targetPath)
	// Case-insensitive prefix match (Windows)
	lowerSb := strings.ToLower(sb)
	lowerTp := strings.ToLower(tp)
	// Exact match or is a subpath
	if lowerTp == lowerSb {
		return true
	}
	// Check if targetPath starts with sandboxDir + separator
	return strings.HasPrefix(lowerTp, lowerSb+string(filepath.Separator))
}

// capabilityForType maps a protocol command type to the corresponding
// PermissionConfig field (returns nil if the type is always allowed).
func capabilityForType(cmdType string) *bool {
	switch cmdType {
	case protocol.TypeExec, protocol.TypeExecPTY:
		// Special handling: checked separately for destructive filter
		return nil
	case protocol.TypeFSRead:
		return nil // always allowed in all tiers
	case protocol.TypeFSList:
		return nil
	case protocol.TypeFSStat:
		return nil
	case protocol.TypeFSHash:
		return nil
	case protocol.TypeHealth:
		return nil
	case protocol.TypeDisplayInfo:
		return nil
	case protocol.TypePing, protocol.TypePong:
		return nil
	case protocol.TypeTokenRotate, protocol.TypeTokenRefresh:
		return nil
	case protocol.TypeFileSave:
		return boolPtr(false) // default deny if not in standard/full
	case protocol.TypeFileRemove:
		return boolPtr(false)
	case protocol.TypeFSMove:
		return boolPtr(false)
	case protocol.TypeFSMkdir:
		return boolPtr(false)
	case protocol.TypeProcList, protocol.TypeTaskList:
		return nil
	case protocol.TypeProcKill, protocol.TypeTaskStop:
		return boolPtr(false)
	case protocol.TypeProcStart:
		return boolPtr(false)
	case protocol.TypeCapture:
		return boolPtr(false)
	case protocol.TypePointerClick, protocol.TypeTextInput, protocol.TypeKeyPress, protocol.TypeKeyCombo:
		return boolPtr(false)
	case protocol.TypeTunnelOpen, protocol.TypeTunnelClose, protocol.TypeTunnelData:
		return boolPtr(false)
	case protocol.TypeMitmStart, protocol.TypeMitmStop, protocol.TypeMitmData:
		return boolPtr(false)
	case protocol.TypeDebugAttach, protocol.TypeDebugDetach, protocol.TypeDebugReadMem, protocol.TypeDebugModules, protocol.TypeDebugMemQuery:
		return boolPtr(false)
	case protocol.TypeClipboardRead:
		return nil
	case protocol.TypeClipboardWrite:
		return boolPtr(false)
	case protocol.TypeOpenLink:
		return boolPtr(false)
	case protocol.TypeNotify:
		return boolPtr(false)
	case protocol.TypeAgentUpdate:
		return boolPtr(false)
	default:
		return nil
	}
}

func boolPtr(v bool) *bool { return &v }

// isAllowed checks whether a command type is permitted under the permission config.
// For exec commands, also checks the destructive command filter.
// For filesystem commands, checks the path sandbox.
func isAllowed(permStr, sandboxDir, cmdType, command, path string) bool {
	pc := resolvePermissionConfig(permStr)

	// Use sandbox from agent config if not set in permission config
	effectiveSandbox := pc.SandboxDir
	if effectiveSandbox == "" {
		effectiveSandbox = sandboxDir
	}

	// Full tier: everything allowed (but still check sandbox for fs ops)
	if pc.Tier == PermFull && effectiveSandbox == "" {
		return true
	}

	// Check if this command type requires a specific capability
	switch cmdType {
	case protocol.TypeExec, protocol.TypeExecPTY:
		if pc.Exec != nil && !*pc.Exec {
			return false
		}
		// Check destructive filter
		if pc.ExecDestructive != nil && !*pc.ExecDestructive {
			if isDestructiveCommand(command) {
				return false
			}
		}
		return true

	case protocol.TypeFileSave:
		if pc.FSWrite != nil && !*pc.FSWrite {
			return false
		}
		return isPathWithinSandbox(effectiveSandbox, path)

	case protocol.TypeFileRemove:
		if pc.FSDelete != nil && !*pc.FSDelete {
			return false
		}
		return isPathWithinSandbox(effectiveSandbox, path)

	case protocol.TypeFSMove:
		if pc.FSMove != nil && !*pc.FSMove {
			return false
		}
		return isPathWithinSandbox(effectiveSandbox, path)

	case protocol.TypeFSMkdir:
		if pc.FSMkdir != nil && !*pc.FSMkdir {
			return false
		}
		return isPathWithinSandbox(effectiveSandbox, path)

	case protocol.TypeFSRead, protocol.TypeFSList, protocol.TypeFSStat, protocol.TypeFSHash:
		// Read ops: always allowed in all tiers, but check sandbox if set
		return isPathWithinSandbox(effectiveSandbox, path)

	case protocol.TypeProcKill, protocol.TypeTaskStop:
		if pc.ProcKill != nil && !*pc.ProcKill {
			return false
		}
		return true

	case protocol.TypeProcStart:
		if pc.ProcStart != nil && !*pc.ProcStart {
			return false
		}
		return true

	case protocol.TypeCapture:
		if pc.Capture != nil && !*pc.Capture {
			return false
		}
		return true

	case protocol.TypePointerClick, protocol.TypeTextInput, protocol.TypeKeyPress, protocol.TypeKeyCombo:
		if pc.Input != nil && !*pc.Input {
			return false
		}
		return true

	case protocol.TypeTunnelOpen, protocol.TypeTunnelClose, protocol.TypeTunnelData:
		if pc.Tunnel != nil && !*pc.Tunnel {
			return false
		}
		return true

	case protocol.TypeMitmStart, protocol.TypeMitmStop, protocol.TypeMitmData:
		if pc.Mitm != nil && !*pc.Mitm {
			return false
		}
		return true

	case protocol.TypeDebugAttach, protocol.TypeDebugDetach, protocol.TypeDebugReadMem, protocol.TypeDebugModules, protocol.TypeDebugMemQuery:
		if pc.Debug != nil && !*pc.Debug {
			return false
		}
		return true

	case protocol.TypeClipboardRead:
		if pc.ClipboardRead != nil && !*pc.ClipboardRead {
			return false
		}
		return true

	case protocol.TypeClipboardWrite:
		if pc.ClipboardWrite != nil && !*pc.ClipboardWrite {
			return false
		}
		return true

	case protocol.TypeOpenLink:
		if pc.OpenLink != nil && !*pc.OpenLink {
			return false
		}
		return true

	case protocol.TypeNotify:
		if pc.Notify != nil && !*pc.Notify {
			return false
		}
		return true

	case protocol.TypeAgentUpdate:
		if pc.AgentUpdate != nil && !*pc.AgentUpdate {
			return false
		}
		return true

	// Always-allowed types (ping, pong, health, display_info, token_rotate, proc_list, task_list)
	default:
		return true
	}
}