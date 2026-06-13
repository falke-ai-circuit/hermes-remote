package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/falke-ai-circuit/hermes-remote/internal/protocol"
	"github.com/gorilla/websocket"
)

// Server is the multi-session WebSocket server with agent registry, LLM proxy, and session management.
type Server struct {
	addr      string
	token     string
	registry  *Registry
	sessions  *SessionManager
	proxy     *LLMProxy
	srv       *http.Server
	mux       *http.ServeMux

	mu        sync.RWMutex
	conns     map[string]*websocket.Conn // agentID -> conn
}

// NewServer creates a new server.
func NewServer(addr string, token string, registryPath string) *Server {
	reg := NewRegistry(registryPath)
	return &Server{
		addr:     addr,
		token:    token,
		registry: reg,
		sessions: NewSessionManager(),
		proxy:    NewLLMProxy(),
		conns:    make(map[string]*websocket.Conn),
	}
}

// Start begins listening for WebSocket and HTTP connections.
func (s *Server) Start() error {
	s.mux = http.NewServeMux()
	s.srv = &http.Server{
		Addr:    s.addr,
		Handler: s.mux,
	}

	// WebSocket endpoint
	s.mux.HandleFunc("/ws", s.handleWebSocket)

	// HTTP API endpoints
	s.mux.HandleFunc("/api/agents", s.handleListAgents)
	s.mux.HandleFunc("/api/agent/", s.handleAgentRoute)

	log.Printf("[server] starting on %s", s.addr)
	return s.srv.ListenAndServe()
}

// StartTLS begins listening with TLS.
func (s *Server) StartTLS(certFile, keyFile string) error {
	s.mux = http.NewServeMux()
	s.srv = &http.Server{
		Addr:    s.addr,
		Handler: s.mux,
	}

	s.mux.HandleFunc("/ws", s.handleWebSocket)
	s.mux.HandleFunc("/api/agents", s.handleListAgents)
	s.mux.HandleFunc("/api/agent/", s.handleAgentRoute)

	log.Printf("[server] starting TLS on %s", s.addr)
	return s.srv.ListenAndServeTLS(certFile, keyFile)
}

// Close shuts down the server.
func (s *Server) Close() error {
	if s.srv != nil {
		return s.srv.Close()
	}
	return nil
}

// handleWebSocket upgrades HTTP to WebSocket, authenticates, and processes agent connections.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Auth check
	authHeader := r.Header.Get("Authorization")
	if s.token != "" {
		if authHeader != "Bearer "+s.token {
			log.Printf("[server] auth rejected from %s", r.RemoteAddr)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	upgrader := &websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[server] upgrade failed: %v", err)
		return
	}

	// Read initial agent info message
	var env protocol.Envelope
	if err := conn.ReadJSON(&env); err != nil {
		log.Printf("[server] failed to read agent info: %v", err)
		conn.Close()
		return
	}

	var info protocol.AgentInfo
	if env.Result != nil {
		if err := json.Unmarshal(env.Result, &info); err != nil {
			log.Printf("[server] failed to parse agent info: %v", err)
			conn.Close()
			return
		}
	}

	hostname, _ := os.Hostname()
	agentID := fmt.Sprintf("a0-%s", hostname)

	// Register agent
	s.registry.Register(agentID, info.Name, info.Version, info.OS, info.Arch, info.Mode)

	// Create session
	s.sessions.CreateSession(agentID)

	// Store connection
	s.mu.Lock()
	s.conns[agentID] = conn
	s.mu.Unlock()

	log.Printf("[server] agent connected: %s (%s/%s, mode=%s)", agentID, info.OS, info.Arch, info.Mode)

	// Handle messages
	go s.handleMessages(agentID, conn)
}

// handleMessages processes incoming WebSocket messages from an agent.
func (s *Server) handleMessages(agentID string, conn *websocket.Conn) {
	defer func() {
		conn.Close()
		s.mu.Lock()
		delete(s.conns, agentID)
		s.mu.Unlock()
		s.registry.Unregister(agentID)
		s.sessions.RemoveSession(agentID)
		log.Printf("[server] agent disconnected: %s", agentID)
	}()

	for {
		var env protocol.Envelope
		if err := conn.ReadJSON(&env); err != nil {
			log.Printf("[server] read error from %s: %v", agentID, err)
			return
		}

		switch env.Type {
		case protocol.TypePing:
			pong := protocol.NewPong(env.ID)
			conn.WriteJSON(pong)
			s.registry.Heartbeat(agentID)

		case protocol.TypePong:
			s.registry.Heartbeat(agentID)

		default:
			// Store results in session context if relevant
			s.sessions.AddMemory(agentID, "last_msg_type", env.Type)
			s.sessions.AddMemory(agentID, fmt.Sprintf("msg_%d", time.Now().UnixMilli()), string(mustMarshalRaw(env)))
		}
	}
}

// handleListAgents returns the list of registered agents as JSON.
func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	agents := s.registry.ListAgents()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agents)
}

// handleAgentRoute dispatches /api/agent/{id}/... routes.
func (s *Server) handleAgentRoute(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/agent/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 {
		http.Error(w, "agent ID required", http.StatusBadRequest)
		return
	}

	agentID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch {
	case action == "shell" && r.Method == http.MethodPost:
		s.handleAgentShell(w, r, agentID)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// handleAgentShell executes a shell command on behalf of an agent.
func (s *Server) handleAgentShell(w http.ResponseWriter, r *http.Request, agentID string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var params protocol.ShellParams
	if err := json.Unmarshal(body, &params); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if params.Timeout <= 0 {
		params.Timeout = 60
	}

	start := time.Now()

	cmd := exec.Command("sh", "-c", params.Command)
	if params.WorkDir != "" {
		cmd.Dir = params.WorkDir
	}
	if len(params.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range params.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	// Apply timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	var timedOut bool
	timeout := time.Duration(params.Timeout) * time.Second
	select {
	case err = <-done:
		timedOut = false
	case <-time.After(timeout):
		cmd.Process.Kill()
		timedOut = true
		<-done
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	result := protocol.ShellResult{
		Stdout:     "", // output captured separately
		Stderr:     "",
		ExitCode:   exitCode,
		DurationMs: time.Since(start).Milliseconds(),
		TimedOut:   timedOut,
	}

	// Store result in session
	s.sessions.AddMemory(agentID, "last_shell", params.Command)
	s.sessions.AddMemory(agentID, fmt.Sprintf("shell_%d", time.Now().UnixMilli()), fmt.Sprintf("exit=%d timed_out=%v", exitCode, timedOut))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func mustMarshalRaw(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}
