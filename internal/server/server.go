package server

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
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

// RateLimitConfig holds the rate-limiter settings supplied from the CLI /
// environment. A zero value falls back to the proxy defaults (10/s, burst 20,
// max 5 concurrent).
type RateLimitConfig struct {
	RatePerSec    float64 // tokens added per second; <=0 → default
	Burst         int     // bucket capacity; <=0 → default
	MaxConcurrent int     // global in-flight cap; <=0 → default
}

// Server is the multi-session WebSocket server with agent registry, LLM proxy, and session management.
type Server struct {
	addr      string
	token     string
	registry  *Registry
	sessions  *SessionManager
	proxy     *LLMProxy
	rateLimit *RateLimiter
	srv       *http.Server
	mux       *http.ServeMux

	// TLS configuration for the server (optional).
	certFile     string
	keyFile      string
	clientCAFile string // optional CA for TLS mutual authentication (mTLS)

	mu        sync.RWMutex
	conns     map[string]*websocket.Conn // agentID -> conn

	tokenTTL    time.Duration                 // configured token TTL (0 = rotation disabled)
	tokenExpiry map[string]time.Time          // agentID -> token expiry time
	tokenMu       sync.Mutex                   // guards tokenExpiry + rotatedTokens
	rotatedTokens map[string]string            // agentID -> last rotated token
	tokenStop   chan struct{}                 // closes to stop rotation goroutine
	tokenWG     sync.WaitGroup                // waits for rotation goroutine on shutdown
}

// startTime records when the server process began, used by /health for uptime.
var startTime = time.Now()

// NewServer creates a new server.
func NewServer(addr string, token string, registryPath string) *Server {
	reg := NewRegistry(registryPath)
	reg.StartStaleDetector()
	return &Server{
		addr:        addr,
		token:       token,
		registry:    reg,
		sessions:    NewSessionManager(),
		proxy:       NewLLMProxy(),
		conns:       make(map[string]*websocket.Conn),
		tokenExpiry:   make(map[string]time.Time),
		rotatedTokens: make(map[string]string),
		tokenStop:   make(chan struct{}),
	}
}

// NewServerWithRateLimit creates a new server with an explicit rate-limit
// configuration applied to the LLM proxy. Pass a RateLimitConfig whose zero
// values fall back to the proxy defaults (10 req/s, burst 20, max 5
// concurrent).
func NewServerWithRateLimit(addr string, token string, registryPath string, rlCfg RateLimitConfig) *Server {
	srv := NewServer(addr, token, registryPath)
	srv.rateLimit = NewRateLimiter(rlCfg.RatePerSec, rlCfg.Burst, rlCfg.MaxConcurrent)
	srv.proxy.SetRateLimiter(srv.rateLimit)
	return srv
}

// NewServerWithTLS creates a new server configured for TLS. certFile/keyFile
// are the server's TLS certificate and key. clientCAFile, when non-empty,
// enables TLS mutual authentication: clients must present a certificate
// signed by that CA or the TLS handshake is rejected. The returned server is
// started with StartTLS.
func NewServerWithTLS(addr string, token string, registryPath string, certFile string, keyFile string, clientCAFile string) *Server {
	srv := NewServer(addr, token, registryPath)
	srv.certFile = certFile
	srv.keyFile = keyFile
	srv.clientCAFile = clientCAFile
	return srv
}

// NewServerWithTLSRateLimit creates a new TLS server with an explicit
// rate-limit configuration. It is the TLS + rate-limiting combination of
// NewServerWithRateLimit and NewServerWithTLS.
func NewServerWithTLSRateLimit(addr string, token string, registryPath string, certFile string, keyFile string, clientCAFile string, rlCfg RateLimitConfig) *Server {
	srv := NewServerWithTLS(addr, token, registryPath, certFile, keyFile, clientCAFile)
	srv.rateLimit = NewRateLimiter(rlCfg.RatePerSec, rlCfg.Burst, rlCfg.MaxConcurrent)
	srv.proxy.SetRateLimiter(srv.rateLimit)
	return srv
}

// SetTokenTTL configures the server-side token rotation interval. When ttl > 0
// the server runs a background goroutine that proactively rotates each
// connected agent's token ttl before it expires. A ttl of 0 disables
// rotation. Must be called before Start/StartTLS.
func (s *Server) SetTokenTTL(ttl time.Duration) {
	s.tokenTTL = ttl
}

// generateToken produces a fresh opaque token for rotation. It uses crypto/rand
// (stdlib) so the token is unpredictable enough for an auth bearer string.
// On the unlikely chance rand.Read fails it falls back to a time-based token so
// rotation never blocks.
func generateToken() string {
	var b [24]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("tok-%d", time.Now().UnixNano())
}

// StartTokenRotation launches the background goroutine that proactively
// rotates tokens before they expire. It is a no-op if tokenTTL is zero. Safe
// to call once; started automatically by Start/StartTLS when tokenTTL > 0.
func (s *Server) StartTokenRotation() {
	if s.tokenTTL <= 0 {
		return
	}
	s.tokenWG.Add(1)
	go s.runTokenRotation()
}

// runTokenRotation scans every minute for agents whose token is close to
// expiry and sends them a new token via InitiateTokenRotation. It records the
// new expiry in the tokenExpiry map so the next rotation is scheduled relative
// to the new token. Exits when tokenStop is closed (in Close).
func (s *Server) runTokenRotation() {
	defer s.tokenWG.Done()
	// Check every minute. rotationLeadTime is how far before expiry we rotate.
	const checkInterval = 60 * time.Second
	const rotationLeadTime = 5 * time.Minute
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.tokenStop:
			return
		case <-ticker.C:
			now := time.Now()
			// Snapshot agents needing rotation under lock, then rotate without holding lock
			type rotateTask struct {
				agentID   string
				newToken  string
			}
			var tasks []rotateTask
			s.tokenMu.Lock()
			for agentID, expiry := range s.tokenExpiry {
				if !expiry.IsZero() && now.Add(rotationLeadTime).After(expiry) {
					tasks = append(tasks, rotateTask{agentID: agentID, newToken: generateToken()})
					// Schedule next expiry relative to now
					s.tokenExpiry[agentID] = now.Add(s.tokenTTL)
				}
			}
			s.tokenMu.Unlock()
			// Rotate without holding tokenMu (InitiateTokenRotation locks it internally)
			for _, t := range tasks {
				if err := s.InitiateTokenRotation(t.agentID, t.newToken); err != nil {
					log.Printf("[server] token rotation failed for agent %s: %v", t.agentID, err)
				} else {
					log.Printf("[server] proactively rotated token for agent %s (next expiry in %v)", t.agentID, s.tokenTTL)
				}
			}
			}
			}
			}

			// SetTokenExpiry records the expiry time for an agent's token. Called when an
// agent connects (the server issues a TTL-based expiry) or after a manual
// rotation. A zero expiry means "no expiry tracking".
func (s *Server) SetTokenExpiry(agentID string, expiry time.Time) {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	if expiry.IsZero() {
		delete(s.tokenExpiry, agentID)
		return
	}
	s.tokenExpiry[agentID] = expiry
}

// ClearTokenExpiry removes the expiry tracking for an agent (called on disconnect).
func (s *Server) ClearTokenExpiry(agentID string) {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	delete(s.tokenExpiry, agentID)
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

	// Health endpoint
	s.mux.HandleFunc("/health", s.handleHealth)

	// HTTP API endpoints
	s.mux.HandleFunc("/api/agents", s.handleListAgents)
	s.mux.HandleFunc("/api/agent/", s.handleAgentRoute)

	// Start proactive token rotation if a TTL was configured.
	s.StartTokenRotation()

	log.Printf("[server] starting on %s", s.addr)
	return s.srv.ListenAndServe()
}

// StartTLS begins listening with TLS. When the server was constructed with a
// clientCAFile (via NewServerWithTLS), the TLS config requires and verifies
// client certificates (mTLS). certFile/keyFile override the server's stored
// cert/key paths when non-empty.
func (s *Server) StartTLS(certFile, keyFile string) error {
	if certFile != "" {
		s.certFile = certFile
	}
	if keyFile != "" {
		s.keyFile = keyFile
	}

	s.mux = http.NewServeMux()

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
	}
	// mTLS: require and verify client certificates when a client CA is configured.
	if s.clientCAFile != "" {
		caCert, err := os.ReadFile(s.clientCAFile)
		if err != nil {
			return fmt.Errorf("read client CA: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return fmt.Errorf("failed to parse client CA")
		}
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		tlsConfig.ClientCAs = caCertPool
	}

	s.srv = &http.Server{
		Addr:      s.addr,
		Handler:   s.mux,
		TLSConfig: tlsConfig,
	}

	s.mux.HandleFunc("/ws", s.handleWebSocket)
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/api/agents", s.handleListAgents)
	s.mux.HandleFunc("/api/agent/", s.handleAgentRoute)

	// Start proactive token rotation if a TTL was configured.
	s.StartTokenRotation()

	mode := "TLS"
	if s.clientCAFile != "" {
		mode = "TLS+mTLS"
	}
	log.Printf("[server] starting %s on %s", mode, s.addr)
	return s.srv.ListenAndServeTLS(s.certFile, s.keyFile)
}

// Close shuts down the server, stops the stale-detector goroutine, and stops
// the token rotation goroutine (if running).
func (s *Server) Close() error {
	s.registry.Stop()
	// Stop the token rotation goroutine and wait for it to drain.
	close(s.tokenStop)
	s.tokenWG.Wait()
	if s.srv != nil {
		return s.srv.Close()
	}
	return nil
}

// handleWebSocket upgrades HTTP to WebSocket, authenticates, and processes agent connections.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Auth check — accept global token OR per-agent rotated token
	authHeader := r.Header.Get("Authorization")
	if s.token != "" {
		valid := authHeader == "Bearer "+s.token
		if !valid {
			// Check rotated tokens
			s.tokenMu.Lock()
			for _, rt := range s.rotatedTokens {
				if authHeader == "Bearer "+rt {
					valid = true
					break
				}
			}
			s.tokenMu.Unlock()
		}
		if !valid {
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

	// Track token expiry so the rotation goroutine can proactively rotate it.
	if s.tokenTTL > 0 {
		s.SetTokenExpiry(agentID, time.Now().Add(s.tokenTTL))
	}

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
		s.ClearTokenExpiry(agentID)
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

		case protocol.TypeHealthResult:
			// Agent reported health; update resource usage and refresh heartbeat.
			if env.Result != nil {
				var hr protocol.HealthResult
				if err := json.Unmarshal(env.Result, &hr); err == nil {
					s.registry.UpdateHealth(agentID, ResourceInfo{
						CPUPercent: hr.CPUPercent,
						MemoryMB:   hr.MemoryMB,
						DiskFreeMB: hr.DiskFreeMB,
					})
				}
			}

		case protocol.TypeError:
			if env.Error != nil {
				s.registry.RecordError(agentID, env.Error.Message)
			}

		case protocol.TypeTokenRotateResult:
			// Agent confirmed it applied a rotated token. Record the event and
			// schedule the next expiry from now (relative to the new token).
			var result protocol.TokenRotateResult
			if env.Result != nil {
				_ = json.Unmarshal(env.Result, &result)
			}
			log.Printf("[server] agent %s rotated token (rotated=%v)", agentID, result.Rotated)
			s.sessions.AddMemory(agentID, "token_rotated_at", time.Now().UTC().Format(time.RFC3339))
			if s.tokenTTL > 0 {
				s.SetTokenExpiry(agentID, time.Now().Add(s.tokenTTL))
			}

		case protocol.TypeTokenRefresh:
			// Agent requested a proactive token refresh (its token is nearing
			// expiry). Generate a new token and send it back.
			newToken := generateToken()
			if err := s.InitiateTokenRotation(agentID, newToken); err != nil {
				log.Printf("[server] proactive refresh failed for agent %s: %v", agentID, err)
			} else {
				log.Printf("[server] sent refreshed token to agent %s", agentID)
				if s.tokenTTL > 0 {
					s.SetTokenExpiry(agentID, time.Now().Add(s.tokenTTL))
				}
			}

		default:
			// Store results in session context if relevant
			s.sessions.AddMemory(agentID, "last_msg_type", env.Type)
			s.sessions.AddMemory(agentID, fmt.Sprintf("msg_%d", time.Now().UnixMilli()), string(mustMarshalRaw(env)))
		}
	}
}

// InitiateTokenRotation sends a token_rotate command to the agent with the new
// token. It is used by the proactive rotation goroutine and can also be called
// manually (e.g. from an admin endpoint). Returns an error if the agent is not
// connected.
func (s *Server) InitiateTokenRotation(agentID string, newToken string) error {
	s.mu.RLock()
	conn, ok := s.conns[agentID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("agent %s not connected", agentID)
	}
	params := protocol.TokenRotateParams{
		NewToken: newToken,
	}
	if s.tokenTTL > 0 {
		params.Expiry = time.Now().Add(s.tokenTTL)
	}
	paramData, _ := json.Marshal(params)
	env := protocol.Envelope{
		ID:     fmt.Sprintf("token-rotate-%d", time.Now().UnixMilli()),
		Type:   protocol.TypeTokenRotate,
		Params: paramData,
	}
	if err := conn.WriteJSON(env); err != nil {
		return fmt.Errorf("send token_rotate: %w", err)
	}
	// Store rotated token so server accepts it on reconnect
	s.tokenMu.Lock()
	s.rotatedTokens[agentID] = newToken
	s.tokenMu.Unlock()
	log.Printf("[server] rotated token stored for agent %s", agentID)
	return nil
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
	case action == "fs-read" && r.Method == http.MethodPost:
		s.handleAgentFSRead(w, r, agentID)
	case action == "fs-write" && r.Method == http.MethodPost:
		s.handleAgentFSWrite(w, r, agentID)
	case action == "screenshot" && r.Method == http.MethodPost:
		s.handleAgentScreenshot(w, r, agentID)
	case action == "process-list" && r.Method == http.MethodPost:
		s.handleAgentProcessList(w, r, agentID)
	case action == "health" && r.Method == http.MethodGet:
		s.handleAgentHealth(w, r, agentID)
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

	// Run command with timeout
	type cmdResult struct {
		output []byte
		err    error
	}
	done := make(chan cmdResult, 1)
	go func() {
		out, err := cmd.CombinedOutput()
		done <- cmdResult{out, err}
	}()

	var output []byte
	var cmdErr error
	timedOut := false

	select {
	case res := <-done:
		output = res.output
		cmdErr = res.err
	case <-time.After(time.Duration(params.Timeout) * time.Second):
		cmd.Process.Kill()
		<-done // wait for goroutine to finish after killing
		timedOut = true
	}

	exitCode := 0
	if cmdErr != nil {
		if exitErr, ok := cmdErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if !timedOut {
			exitCode = -1
		}
	}

	result := protocol.ShellResult{
		Stdout:     string(output),
		Stderr:     "",
		ExitCode:   exitCode,
		DurationMs: time.Since(start).Milliseconds(),
		TimedOut:   timedOut,
	}

	// Store result in session
	s.sessions.AddMemory(agentID, "last_shell", params.Command)
	s.sessions.AddMemory(agentID, fmt.Sprintf("shell_%d", time.Now().UnixMilli()), fmt.Sprintf("exit=%d", exitCode))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func mustMarshalRaw(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}

// handleAgentFSRead reads a file on behalf of an agent.
func (s *Server) handleAgentFSRead(w http.ResponseWriter, r *http.Request, agentID string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var params protocol.FSParams
	if err := json.Unmarshal(body, &params); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	data, err := os.ReadFile(params.Path)
	if err != nil {
		http.Error(w, fmt.Sprintf("read failed: %v", err), http.StatusInternalServerError)
		return
	}

	if params.Offset > 0 && params.Offset < len(data) {
		data = data[params.Offset:]
	}
	if params.Limit > 0 && params.Limit < len(data) {
		data = data[:params.Limit]
	}

	result := protocol.FSReadResult{
		Data:     base64.StdEncoding.EncodeToString(data),
		Size:     int64(len(data)),
		Encoding: "base64",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleAgentFSWrite writes a file on behalf of an agent.
func (s *Server) handleAgentFSWrite(w http.ResponseWriter, r *http.Request, agentID string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var params protocol.FSParams
	if err := json.Unmarshal(body, &params); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	raw, err := base64.StdEncoding.DecodeString(params.Data)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid base64: %v", err), http.StatusBadRequest)
		return
	}

	if err := os.WriteFile(params.Path, raw, 0644); err != nil {
		http.Error(w, fmt.Sprintf("write failed: %v", err), http.StatusInternalServerError)
		return
	}

	result := protocol.FSWriteResult{Written: len(raw), Path: params.Path}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleAgentScreenshot captures a screenshot on behalf of an agent.
func (s *Server) handleAgentScreenshot(w http.ResponseWriter, r *http.Request, agentID string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var params protocol.ScreenParams
	if err := json.Unmarshal(body, &params); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Try import (ImageMagick), fallback to scrot
	cmd := exec.Command("import", "-window", "root", "png:-")
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		cmd2 := exec.Command("scrot", "-")
		cmd2.Env = os.Environ()
		out, err = cmd2.Output()
		if err != nil {
			result := protocol.ScreenshotResult{
				Format:    "error",
				Width:     0,
				Height:    0,
				Data:      "",
				SizeBytes: 0,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result)
			return
		}
	}

	result := protocol.ScreenshotResult{
		Format:    "png",
		Width:     0,
		Height:    0,
		Data:      base64.StdEncoding.EncodeToString(out),
		SizeBytes: int64(len(out)),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleAgentProcessList returns the list of running processes on behalf of an agent.
func (s *Server) handleAgentProcessList(w http.ResponseWriter, r *http.Request, agentID string) {
	// Use ps for cross-platform compatibility
	cmd := exec.Command("ps", "-eo", "pid,comm,%cpu,%mem", "--no-headers")
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		http.Error(w, fmt.Sprintf("ps failed: %v", err), http.StatusInternalServerError)
		return
	}

	var processes []protocol.ProcessInfo
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pid := 0
		fmt.Sscanf(fields[0], "%d", &pid)
		cpu := 0.0
		fmt.Sscanf(fields[2], "%f", &cpu)
		mem := 0.0
		fmt.Sscanf(fields[3], "%f", &mem)
		processes = append(processes, protocol.ProcessInfo{
			PID:        pid,
			Name:       fields[1],
			CPUPercent: cpu,
			MemoryMB:   mem,
		})
	}

	result := protocol.ProcessListResult{Processes: processes}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleAgentHealth returns the full health record for a single agent.
func (s *Server) handleAgentHealth(w http.ResponseWriter, r *http.Request, agentID string) {
	rec, err := s.registry.GetHealth(agentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rec)
}

// handleHealth returns a server health check with per-server stats.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	agents := s.registry.ListAgents()
	total := len(agents)
	active := 0
	stale := 0
	for _, a := range agents {
		switch a.Status {
		case "active":
			active++
		case "stale":
			stale++
		}
	}
	resp := map[string]interface{}{
		"status":        "ok",
		"total_agents":  total,
		"active_agents": active,
		"stale_agents":  stale,
		"uptime_seconds": int64(time.Since(startTime).Seconds()),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
