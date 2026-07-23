package server

import (
	"testing"

	"github.com/falke-ai-circuit/probe/internal/protocol"
)

// TestCapabilityForCommand_Phase7 verifies that the capability mapping
// returns the correct capability for each new Phase 7 command type.
func TestCapabilityForCommand_Phase7(t *testing.T) {
	cases := []struct {
		msgType    string
		capability string
	}{
		{protocol.TypeSocks5Start, "socks5"},
		{protocol.TypeSocks5Stop, "socks5"},
		{protocol.TypePortForward, "port_forward"},
		{protocol.TypePortScan, "port_scan"},
		{protocol.TypeNetConnections, "net_info"},
		{protocol.TypeAutostartEnable, "autostart"},
		{protocol.TypeAutostartDisable, "autostart"},
		{protocol.TypeAutostartStatus, "autostart"},
		{protocol.TypeFileSearch, "file_search"},
		{protocol.TypeSysInfo, "sysinfo"},
	}

	for _, c := range cases {
		cap := capabilityForCommand(c.msgType)
		if cap != c.capability {
			t.Errorf("capabilityForCommand(%q): got %q, want %q", c.msgType, cap, c.capability)
		}
	}
}

// TestHasCapability_Phase7 verifies HasCapability works correctly for new capabilities.
func TestHasCapability_Phase7(t *testing.T) {
	// Agent with specific Phase 7 capabilities
	agentCaps := []string{"socks5", "port_scan", "sysinfo", "net_info", "file_search"}

	// Should have these
	for _, cap := range agentCaps {
		if !HasCapability(agentCaps, cap) {
			t.Errorf("expected HasCapability(%q) = true", cap)
		}
	}

	// Should NOT have these
	for _, cap := range []string{"port_forward", "autostart", "tunnel", "exec"} {
		if HasCapability(agentCaps, cap) {
			t.Errorf("expected HasCapability(%q) = false", cap)
		}
	}
}

// TestCheckAgentCapability_Phase7 verifies CheckAgentCapability returns an
// error when the agent lacks a required Phase 7 capability.
func TestCheckAgentCapability_Phase7(t *testing.T) {
	// Agent has socks5 but not port_forward
	agentCaps := []string{"socks5", "sysinfo"}

	// socks5_start should be allowed
	if err := CheckAgentCapability("agent-1", agentCaps, protocol.TypeSocks5Start); err != nil {
		t.Errorf("expected nil error for socks5_start, got: %v", err)
	}

	// port_forward should be denied
	if err := CheckAgentCapability("agent-1", agentCaps, protocol.TypePortForward); err == nil {
		t.Error("expected error for port_forward (not in caps), got nil")
	}

	// sysinfo should be allowed
	if err := CheckAgentCapability("agent-1", agentCaps, protocol.TypeSysInfo); err != nil {
		t.Errorf("expected nil error for sysinfo, got: %v", err)
	}

	// Empty caps = backward compat = all allowed
	if err := CheckAgentCapability("agent-1", []string{}, protocol.TypePortForward); err != nil {
		t.Errorf("expected nil error for port_forward (empty caps = all allowed), got: %v", err)
	}
}