package agent

import (
	"strings"

	"github.com/falke-ai-circuit/hermes-remote/internal/protocol"
)

// Permission tier constants
const (
	PermReadOnly  = "read-only"
	PermStandard  = "standard"
	PermFull      = "full"
)

// readOnlyCommands are the command types allowed in read-only mode.
var readOnlyCommands = map[string]bool{
	protocol.TypeFSRead:    true,
	protocol.TypeFSList:    true,
	protocol.TypeFSStat:    true,
	protocol.TypeFSHash:    true,
	protocol.TypePing:      true,
	protocol.TypePong:      true,
	protocol.TypeHealth:    true,
	protocol.TypeDisplayInfo: true,
	protocol.TypeExec:      true, // exec is allowed but destructive commands are filtered
}

// standardCommands adds write/execute capabilities on top of read-only.
var standardCommands = map[string]bool{
	protocol.TypeFSRead:    true,
	protocol.TypeFSList:    true,
	protocol.TypeFSStat:    true,
	protocol.TypeFSHash:    true,
	protocol.TypeFileSave:  true,
	protocol.TypeFSMkdir:   true,
	protocol.TypeFSMove:    true,
	protocol.TypePing:      true,
	protocol.TypePong:      true,
	protocol.TypeHealth:    true,
	protocol.TypeDisplayInfo: true,
	protocol.TypeExec:      true,
	protocol.TypeExecPTY:   true,
	protocol.TypeTaskList:  true,
	protocol.TypeTaskStop:  true,
	protocol.TypeClipboardRead: true,
	protocol.TypeProcList:  true,
}

// destructiveExecPatterns are command patterns blocked in read-only exec mode.
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

// isAllowed checks whether a command type is permitted under the given permission tier.
// For exec commands, it also checks if the command itself is destructive (read-only mode only).
func isAllowed(permissions, cmdType, command string) bool {
	switch permissions {
	case PermFull, "":
		return true
	case PermReadOnly:
		if !readOnlyCommands[cmdType] {
			return false
		}
		// For exec commands, check if the command is destructive
		if cmdType == protocol.TypeExec || cmdType == protocol.TypeExecPTY {
			return !isDestructiveCommand(command)
		}
		return true
	case PermStandard:
		return standardCommands[cmdType]
	default:
		// Unknown tier → full access (backward compatible)
		return true
	}
}