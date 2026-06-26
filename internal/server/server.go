package server

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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
	token     string   // primary auth token
	tokens    []string // all accepted tokens (for multi-token rollover support)
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

	// pendingRequests maps request IDs to response channels for request-response
	// over WebSocket. When handleAgentExec sends a command to an agent, it creates
	// a channel and waits on it. handleMessages delivers the response.
	pendingMu    sync.Mutex
	pendingReqs  map[string]chan protocol.Envelope // requestID -> response channel

	// Tunnels: server-side TCP listeners that relay through WebSocket to the agent.
	tunnelMu    sync.Mutex
	tunnels     map[string]*Tunnel // tunnelID -> Tunnel
	tunnelCount int               // for generating unique tunnel IDs

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
		addr:          addr,
		token:         token,
		registry:      reg,
		sessions:      NewSessionManager(),
		proxy:         NewLLMProxy(),
		conns:         make(map[string]*websocket.Conn),
		pendingReqs:   make(map[string]chan protocol.Envelope),
		tunnels:       make(map[string]*Tunnel),
		tokenExpiry:   make(map[string]time.Time),
		rotatedTokens: make(map[string]string),
		tokenStop:     make(chan struct{}),
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

// SetExtraTokens configures additional accepted auth tokens. This enables
// safe deployment rollover: start a new server with a new primary token plus
// the old token as an extra, so the old agent (still using the old token)
// can connect to the new server until its config is updated.
func (s *Server) SetExtraTokens(extra []string) {
	s.tokens = append(s.tokens, s.token)
	s.tokens = append(s.tokens, extra...)
}

// isValidToken checks whether the given bearer token matches any accepted token.
func (s *Server) isValidToken(authHeader string) bool {
	if s.token == "" {
		return true // no auth configured
	}
	// Check primary token
	if authHeader == "Bearer "+s.token {
		return true
	}
	// Check extra tokens
	for _, t := range s.tokens {
		if authHeader == "Bearer "+t {
			return true
		}
	}
	// Check rotated tokens
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	for _, rt := range s.rotatedTokens {
		if authHeader == "Bearer "+rt {
			return true
		}
	}
	return false
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
	// File download endpoint (serves files from /tmp/hermes-remote-files/)
	s.mux.HandleFunc("/download/", s.handleFileDownload)

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
	s.mux.HandleFunc("/download/", s.handleFileDownload)

	// Start proactive token rotation if a TTL is configured.
	s.StartTokenRotation()

	mode := "TLS"
	if s.clientCAFile != "" {
		mode = "TLS+mTLS"
	}
	log.Printf("[server] starting %s on %s", mode, s.addr)
	return s.srv.ListenAndServeTLS(s.certFile, s.keyFile)
}

// Close shuts down the server, stops the stale-detector goroutine, and stops
// the token rotation goroutine (if running). Closes all tunnels.
func (s *Server) Close() error {
	// Close all tunnels
	s.tunnelMu.Lock()
	for _, t := range s.tunnels {
		t.Close()
	}
	s.tunnels = make(map[string]*Tunnel)
	s.tunnelMu.Unlock()

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
	// Auth check — accept primary, extra, or rotated tokens
	authHeader := r.Header.Get("Authorization")
	if !s.isValidToken(authHeader) {
		log.Printf("[server] auth rejected from %s", r.RemoteAddr)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
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

		case protocol.TypeAuthRefreshResult:
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

		case protocol.TypeAuthRequest:
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
			// Check if this is a response to a pending request (exec, fs_read, etc.)
			s.pendingMu.Lock()
			if ch, ok := s.pendingReqs[env.ID]; ok {
				delete(s.pendingReqs, env.ID)
				s.pendingMu.Unlock()
				ch <- env
				continue
			}
			s.pendingMu.Unlock()

			// Handle tunnel data from agent (target→client direction)
			if env.Type == protocol.TypeTunnelData {
				s.handleTunnelDataFromAgent(env)
				continue
			}

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
		Type:   protocol.TypeAuthRefresh,
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
	case action == "exec" && r.Method == http.MethodPost:
		s.handleAgentExec(w, r, agentID)
	case action == "fs-read" && r.Method == http.MethodPost:
		s.handleAgentFSRead(w, r, agentID)
	case action == "fs-write" && r.Method == http.MethodPost:
		s.handleAgentFSWrite(w, r, agentID)
	case action == "capture" && r.Method == http.MethodPost:
		s.handleAgentCapture(w, r, agentID)
	case action == "task-list" && r.Method == http.MethodPost:
		s.handleAgentTaskList(w, r, agentID)
	case action == "proc-list" && r.Method == http.MethodPost:
		s.handleAgentProcList(w, r, agentID)
	case action == "proc-kill" && r.Method == http.MethodPost:
		s.handleAgentProcKill(w, r, agentID)
	case action == "proc-start" && r.Method == http.MethodPost:
		s.handleAgentProcStart(w, r, agentID)
	case action == "tunnel" && r.Method == http.MethodPost:
		s.handleAgentTunnel(w, r, agentID)
	case action == "tunnel-close" && r.Method == http.MethodPost:
		s.handleAgentTunnelClose(w, r, agentID)
	case action == "sniff" && r.Method == http.MethodPost:
		s.handleAgentSniff(w, r, agentID)
	case action == "sniff-stop" && r.Method == http.MethodPost:
		s.handleAgentSniffStop(w, r, agentID)
	case action == "health" && r.Method == http.MethodGet:
		s.handleAgentHealth(w, r, agentID)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// handleAgentExec executes a shell command ON THE REMOTE AGENT via WebSocket.
// It forwards the command to the connected agent and waits for the response.
func (s *Server) handleAgentExec(w http.ResponseWriter, r *http.Request, agentID string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var params protocol.ExecParams
	if err := json.Unmarshal(body, &params); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if params.Timeout <= 0 {
		params.Timeout = 60
	}

	// Get the agent's WebSocket connection
	s.mu.RLock()
	conn, ok := s.conns[agentID]
	s.mu.RUnlock()
	if !ok {
		http.Error(w, fmt.Sprintf("agent %s not connected", agentID), http.StatusServiceUnavailable)
		return
	}

	// Build the command envelope
	reqID := fmt.Sprintf("exec-%d", time.Now().UnixMilli())
	paramData, _ := json.Marshal(params)
	env := protocol.Envelope{
		ID:     reqID,
		Type:   protocol.TypeExec,
		Params: paramData,
	}

	// Register a pending response channel
	respCh := make(chan protocol.Envelope, 1)
	s.pendingMu.Lock()
	s.pendingReqs[reqID] = respCh
	s.pendingMu.Unlock()
	defer func() {
		s.pendingMu.Lock()
		delete(s.pendingReqs, reqID)
		s.pendingMu.Unlock()
	}()

	// Send the command to the agent
	if err := conn.WriteJSON(env); err != nil {
		http.Error(w, fmt.Sprintf("send to agent failed: %v", err), http.StatusServiceUnavailable)
		return
	}

	// Wait for response with timeout
	select {
	case resp := <-respCh:
		if resp.Error != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"stdout":     "",
				"stderr":     resp.Error.Message,
				"exit_code":  -1,
				"duration_ms": 0,
				"timed_out":  false,
			})
			return
		}
		// Parse the exec result from the response
		var execResult protocol.ExecResult
		if resp.Result != nil {
			json.Unmarshal(resp.Result, &execResult)
		}
		s.sessions.AddMemory(agentID, "last_exec", params.Command)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(execResult)

	case <-time.After(time.Duration(params.Timeout) * time.Second):
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"stdout":     "",
			"stderr":     "command timed out",
			"exit_code":  -1,
			"duration_ms": params.Timeout * 1000,
			"timed_out":  true,
		})
	}
}

func mustMarshalRaw(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}

// handleAgentFSRead reads a file ON THE REMOTE AGENT via WebSocket.
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

	resp, err := s.forwardToAgent(agentID, protocol.TypeFSRead, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleAgentFSWrite writes a file ON THE REMOTE AGENT via WebSocket.
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

	resp, err := s.forwardToAgent(agentID, protocol.TypeFileSave, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleAgentCapture captures display ON THE REMOTE AGENT via WebSocket.
func (s *Server) handleAgentCapture(w http.ResponseWriter, r *http.Request, agentID string) {
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

	resp, err := s.forwardToAgent(agentID, protocol.TypeCapture, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleAgentTaskList lists processes ON THE REMOTE AGENT via WebSocket.
func (s *Server) handleAgentTaskList(w http.ResponseWriter, r *http.Request, agentID string) {
	resp, err := s.forwardToAgent(agentID, protocol.TypeTaskList, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleAgentProcList lists processes ON THE REMOTE AGENT via WebSocket (new).
func (s *Server) handleAgentProcList(w http.ResponseWriter, r *http.Request, agentID string) {
	resp, err := s.forwardToAgent(agentID, protocol.TypeProcList, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleAgentProcKill kills a process ON THE REMOTE AGENT via WebSocket.
func (s *Server) handleAgentProcKill(w http.ResponseWriter, r *http.Request, agentID string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var params protocol.TaskStopParams
	if err := json.Unmarshal(body, &params); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.forwardToAgent(agentID, protocol.TypeProcKill, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleAgentProcStart starts a process ON THE REMOTE AGENT via WebSocket.
func (s *Server) handleAgentProcStart(w http.ResponseWriter, r *http.Request, agentID string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var params protocol.ProcStartParams
	if err := json.Unmarshal(body, &params); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.forwardToAgent(agentID, protocol.TypeProcStart, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// forwardToAgent sends a command to the agent via WebSocket and waits for response.
// This is the generic request-response pattern used by all forwarded handlers.
func (s *Server) forwardToAgent(agentID string, msgType string, params interface{}) (interface{}, error) {
	s.mu.RLock()
	conn, ok := s.conns[agentID]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("agent %s not connected", agentID)
	}

	reqID := fmt.Sprintf("%s-%d", msgType, time.Now().UnixMilli())
	var paramData json.RawMessage
	if params != nil {
		paramData, _ = json.Marshal(params)
	}
	env := protocol.Envelope{
		ID:     reqID,
		Type:   msgType,
		Params: paramData,
	}

	respCh := make(chan protocol.Envelope, 1)
	s.pendingMu.Lock()
	s.pendingReqs[reqID] = respCh
	s.pendingMu.Unlock()
	defer func() {
		s.pendingMu.Lock()
		delete(s.pendingReqs, reqID)
		s.pendingMu.Unlock()
	}()

	if err := conn.WriteJSON(env); err != nil {
		return nil, fmt.Errorf("send to agent failed: %w", err)
	}

	select {
	case resp := <-respCh:
		if resp.Error != nil {
			return map[string]interface{}{
				"error": resp.Error.Message,
			}, nil
		}
		var result interface{}
		if resp.Result != nil {
			json.Unmarshal(resp.Result, &result)
		}
		return result, nil
	case <-time.After(120 * time.Second):
		return nil, fmt.Errorf("agent did not respond within 120s")
	}
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

// handleFileDownload serves files from /tmp/hermes-remote-files/ over HTTP.
// This allows the agent to download large files (like updated binaries) from
// the server without hitting command-line length limits.
// GET /download/{filename}
func (s *Server) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/download/")
	if filename == "" || strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}
	filepath := "/tmp/hermes-remote-files/" + filename
	http.ServeFile(w, r, filepath)
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
