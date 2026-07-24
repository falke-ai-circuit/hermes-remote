package main

import (
	"path/filepath"
	"testing"
)

func TestIsServerCmd(t *testing.T) {
	cases := []struct {
		path       string
		expectSkip bool
	}{
		// Legacy separate binaries
		{"cmd/probe-server/main.go", true},
		{"cmd/logreport-server/main.go", true},
		{"cmd/server/main.go", true},
		{"cmd/probe-client/main.go", false},
		// Unified binary
		{"cmd/probe/main.go", false},
		{"cmd/probe/connect.go", false},
		{"cmd/probe/serve.go", true},
		{"cmd/probe/relay.go", true},
		{"cmd/probe/serve_stub.go", false},
		{"cmd/probe/relay_stub.go", false},
		// Non-cmd directories
		{"internal/server/server.go", false},
		{"internal/agent/agent.go", false},
		{"tools/obfuscate/main.go", false},
	}
	for _, tc := range cases {
		// Normalize to OS separator
		p := filepath.FromSlash(tc.path)
		got := isServerCmd(p)
		if got != tc.expectSkip {
			t.Errorf("isServerCmd(%q) = %v, want %v", tc.path, got, tc.expectSkip)
		}
	}
}