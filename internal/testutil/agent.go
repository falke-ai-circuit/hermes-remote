package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/falke-ai-circuit/probe/internal/protocol"
	"github.com/gorilla/websocket"
)

// TestAgent is a lightweight WebSocket client that connects to a Probe
// server, sends agent info, and provides helper methods to send/receive
// protocol envelopes. It is intentionally minimal — it does NOT use the
// full agent.Agent type (which has platform dependencies and blocking
// reconnect loops). Instead it wraps a raw websocket.Conn so tests have
// full control over the protocol layer.
type TestAgent struct {
	conn   *websocket.Conn
	name   string
	token  string
	server string
}

// NewTestAgent dials a Probe server at serverURL using the given token,
// sends the initial agent_info handshake, and returns a connected
// TestAgent. The cleanup function closes the WebSocket and is registered
// via t.Cleanup().
func NewTestAgent(t *testing.T, serverURL, token string) *TestAgent {
	t.Helper()

	// Convert ws://host:port to host:port for dialing
	hostPort := serverURL
	if len(hostPort) > 5 && hostPort[:5] == "ws://" {
		hostPort = hostPort[5:]
	}
	wsURL := fmt.Sprintf("ws://%s/ws", hostPort)

	header := make(map[string][]string)
	header["Authorization"] = []string{"Bearer " + token}

	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
		NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, 5*time.Second)
		},
	}

	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to dial test server %s: %v", wsURL, err)
	}

	ta := &TestAgent{
		conn:   conn,
		token:  token,
		server: serverURL,
		name:   "test-agent",
	}

	// Send agent_info handshake (the server expects this as the first message)
	info := protocol.AgentInfo{
		Name:            "test-agent",
		Version:         "test-0.1",
		OS:              "linux",
		Arch:            "amd64",
		Mode:            "outbound",
		ProtocolVersion: "2",
	}
	infoData, _ := json.Marshal(info)
	handshake := protocol.Envelope{
		ID:     "agent-info",
		Type:   "agent_info",
		Result: infoData,
	}
	if err := conn.WriteJSON(handshake); err != nil {
		conn.Close()
		t.Fatalf("failed to send agent_info handshake: %v", err)
	}

	t.Cleanup(func() {
		_ = ta.conn.Close()
	})

	return ta
}

// Send sends a protocol envelope to the server.
func (ta *TestAgent) Send(env protocol.Envelope) error {
	return ta.conn.WriteJSON(env)
}

// Recv reads a protocol envelope from the server with a timeout.
func (ta *TestAgent) Recv(timeout time.Duration) (protocol.Envelope, error) {
	var env protocol.Envelope
	if err := ta.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return env, err
	}
	defer ta.conn.SetReadDeadline(time.Time{})
	err := ta.conn.ReadJSON(&env)
	return env, err
}

// Conn returns the underlying WebSocket connection for low-level operations.
func (ta *TestAgent) Conn() *websocket.Conn {
	return ta.conn
}

// SendPing sends a ping envelope and returns the pong (or error).
func (ta *TestAgent) SendPing(timeout time.Duration) (protocol.Envelope, error) {
	ping := protocol.NewPing()
	if err := ta.Send(ping); err != nil {
		return protocol.Envelope{}, err
	}
	return ta.Recv(timeout)
}