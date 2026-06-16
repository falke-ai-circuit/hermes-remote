package protocol

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

// Server wraps the HTTP server with WebSocket handling.
type Server struct {
	srv          *http.Server
	mux          *http.ServeMux
	certFile     string
	keyFile      string
	clientCAFile string // optional CA for TLS mutual authentication (mTLS)
	// OnConnect is called when a new WebSocket connection is established.
	// Return true to accept, false to reject.
	OnConnect func(conn *websocket.Conn, r *http.Request) bool
}

// NewServer creates a new WebSocket server wrapper. clientCAFile enables TLS
// mutual authentication when non-empty (the server will require and verify
// client certificates signed by that CA).
func NewServer(addr string, certFile string, keyFile string, clientCAFile string) (*Server, error) {
	srv, mux, err := Listen(addr, certFile, keyFile, clientCAFile)
	if err != nil {
		return nil, err
	}
	return &Server{srv: srv, mux: mux, certFile: certFile, keyFile: keyFile, clientCAFile: clientCAFile}, nil
}

// ListenAndServe starts the WebSocket server. When a cert and key were
// configured it serves over TLS (with mTLS if a client CA was provided);
// otherwise it serves plain HTTP.
func (s *Server) ListenAndServe() error {
	s.mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := GetUpgrader().Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, fmt.Sprintf("upgrade failed: %v", err), http.StatusBadRequest)
			return
		}
		if s.OnConnect != nil && !s.OnConnect(conn, r) {
			conn.Close()
			return
		}
	})
	if s.certFile != "" && s.keyFile != "" {
		return s.srv.ListenAndServeTLS(s.certFile, s.keyFile)
	}
	return s.srv.ListenAndServe()
}

// Close closes the server.
func (s *Server) Close() error {
	return s.srv.Close()
}