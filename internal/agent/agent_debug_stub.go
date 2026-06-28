//go:build !windows

package agent

import (
	"encoding/json"
	"fmt"

	"github.com/falke-ai-circuit/hermes-remote/internal/protocol"
)

// debugManager is a no-op on non-Windows platforms.
type debugManager struct{}

func newDebugManager() *debugManager {
	return &debugManager{}
}

func (a *Agent) handleDebugAttach(env protocol.Envelope) protocol.Envelope {
	var params protocol.DebugAttachParams
	_ = json.Unmarshal(env.Params, &params)
	_ = params
	return protocol.NewError(env.ID, protocol.ErrPlatformNotSupported, "debugger only supported on Windows")
}

func (a *Agent) handleDebugDetach(env protocol.Envelope) protocol.Envelope {
	return protocol.NewError(env.ID, protocol.ErrPlatformNotSupported, "debugger only supported on Windows")
}

func (a *Agent) handleDebugReadMem(env protocol.Envelope) protocol.Envelope {
	return protocol.NewError(env.ID, protocol.ErrPlatformNotSupported, "debugger only supported on Windows")
}

func (a *Agent) handleDebugModules(env protocol.Envelope) protocol.Envelope {
	return protocol.NewError(env.ID, protocol.ErrPlatformNotSupported, "debugger only supported on Windows")
}

func (a *Agent) handleDebugMemQuery(env protocol.Envelope) protocol.Envelope {
	return protocol.NewError(env.ID, protocol.ErrPlatformNotSupported, "debugger only supported on Windows")
}

func (a *Agent) closeAllDebug() {}

func findProcessByName(name string) (int, error) {
	return 0, fmt.Errorf("process lookup not supported on this platform")
}