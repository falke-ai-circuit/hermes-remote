package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"os"
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

	// requireAPIAuth enforces bearer-token auth on HTTP API endpoints
	// (/api/agents, /api/agent/*, /download/*). When false (default), requests
	// without an Authorization header are allowed through with a warning log.
	requireAPIAuth bool

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

// registerRoutes sets up all HTTP routes on the server's mux. Called once
// by both Start and StartTLS to avoid route registration duplication.
func (s *Server) registerRoutes() {
	// WebSocket endpoint
	s.mux.HandleFunc("/ws", s.handleWebSocket)
	// Health endpoint
	s.mux.HandleFunc("/health", s.handleHealth)
	// HTTP API endpoints
	s.mux.HandleFunc("/api/agents", s.handleListAgents)
	s.mux.HandleFunc("/api/agent/", s.handleAgentRoute)
	// File download endpoint (serves files from /tmp/hermes-remote-files/)
	s.mux.HandleFunc("/download/", s.handleFileDownload)
	// Also serve downloads under /api/download/ for Docker proxy compatibility
	s.mux.HandleFunc("/api/download/", s.handleFileDownload)
	s.mux.HandleFunc("/logreport/", s.handleLogReportProxy)
}

// Start begins listening for WebSocket and HTTP connections.
func (s *Server) Start() error {
	s.mux = http.NewServeMux()
	s.srv = &http.Server{
		Addr:    s.addr,
		Handler: s.mux,
	}

	s.registerRoutes()

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

	s.registerRoutes()

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

