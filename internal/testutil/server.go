package testutil

import (
	"fmt"
	"net"
	"testing"

	"github.com/falke-ai-circuit/probe/internal/server"
)

// NewTestServer starts a real Probe server on an ephemeral port (:0).
// It returns the server's WebSocket base URL (ws://host:port) and
// registers cleanup via t.Cleanup().
//
// The server uses a temp file for its registry (t.TempDir) so disk
// persistence doesn't interfere with parallel tests.
func NewTestServer(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	registryPath := dir + "/registry.json"

	// Grab an ephemeral port, then release it so the server can bind.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get ephemeral port: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	srv := server.NewServer(addr, "test-token", registryPath)

	go func() {
		_ = srv.Start()
	}()

	url := fmt.Sprintf("ws://%s", addr)

	t.Cleanup(func() {
		_ = srv.Close()
	})

	return url
}