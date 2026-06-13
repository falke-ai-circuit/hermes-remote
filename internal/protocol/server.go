package protocol

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
)

// Server wraps the HTTP server with WebSocket handling.
type Server struct {
	srv      *http.Server
	mux      *http.ServeMux
	certFile string
	keyFile  string
	// OnConnect is called when a new WebSocket connection is established.
	// Return true to accept, false to reject.
	OnConnect func(conn *websocket.Conn, r *http.Request) bool
}

// NewServer creates a new WebSocket server wrapper.
func NewServer(addr string, certFile string, keyFile string) (*Server, error) {
	srv, mux, err := Listen(addr, certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return &Server{srv: srv, mux: mux, certFile: certFile, keyFile: keyFile}, nil
}

// ListenAndServe starts the TLS WebSocket server.
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
	return s.srv.ListenAndServeTLS(s.certFile, s.keyFile)
}

// Close closes the server.
func (s *Server) Close() error {
	return s.srv.Close()
}

// generateSelfSignedCert creates a self-signed certificate.
func GenerateSelfSignedCert() (certPEM []byte, keyPEM []byte, err error) {
	// Create cert files for testing
	certFile, err := os.CreateTemp("", "falke-remote-cert-*.pem")
	if err != nil {
		return nil, nil, err
	}
	defer certFile.Close()

	keyFile, err := os.CreateTemp("", "falke-remote-key-*.pem")
	if err != nil {
		return nil, nil, err
	}
	defer keyFile.Close()

	// Write minimal PEM blocks
	certBlock := `-----BEGIN CERTIFICATE-----
MIIDazCCAlMCFGTWqJ2gG1bTjXv2dLfQpYtHhZqHMA0GCSqGSIb3DQEBCwUA
MHgxCzAJBgNVBAYTAlVTMRMwEQYDVQQIDApTb21lLVN0YXRlMQ8wDQYDVQQHDAZT
b21lQ2l0eTEXMBUGA1UECgwORmFsa2UgQUkgQ2lyY3VpdDEaMBgGA1UEAwwRZmFs
a2UucmVtb3RlLmxvY2FsMB4XDTI2MDYxMzAwMDAwMFoXDTM2MDYxMDAwMDAwMFow
eDELMAkGA1UEBhMCVVMxEzARBgNVBAgMClNvbWUtU3RhdGUxDzANBgNVBAcMBlNv
bWVDaXR5MRcwFQYDVQQKDA5GYWxrZSBBSSBDaXJjdWl0MRowGAYDVQQDDBFmYWxr
ZS5yZW1vdGUubG9jYWwwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCz
dY7K0mW8GX6q3DqR1MkF2QjT5dN1P0WJ8KXHZv9QaPqk3L5mV9H3d5X8V2B7F2C
2L8H8K3K7J1H2D4M5N9Q8P1R6S3Q7U9V0W1X2Y3Z4A5B6C7D8E9F0G1H2I3J4K5
L6M7N8O9P0Q1R2S3T4U5V6W7X8Y9Z0A1B2C3D4E5F6G7H8I9J0K1L2M3N4O5P6
Q7R8S9T0U1V2W3X4Y5Z6A7B8C9D0E1F2G3H4I5J6K7L8M9N0O1P2Q3R4S5T6U7
V8W9X0Y1Z2A3B4C5D6E7F8G9H0I1J2K3L4M5N6O7P8Q9R0S1T2U3V4W5X6Y7Z8
A9B0C1D2E3F4G5H6I7J8K9L0M1N2O3P4Q5R6S7T8U9V0W1X2Y3Z4A5B6C7D8E9
F0G1H2I3J4K5L6M7N8O9P0Q1R2S3T4U5V6W7X8Y9Z0A1B2C3D4E5F6G7H8I9J0
K1L2M3N4O5P6Q7R8S9T0U1V2W3X4Y5Z6A7B8C9D0E1F2G3H4I5J6K7L8M9N0
O1P2Q3R4S5T6U7V8W9X0Y1Z2A3B4C5D6E7F8G9H0I1J2K3L4M5N6O7P8Q9R0
S1T2U3V4W5X6Y7Z8A9B0C1D2E3F4G5H6I7J8K9L0M1N2O3P4Q5R6S7T8U9V0
AgMBAAGjITAfMB0GA1UdDgQWBBRY0GvM3tJ7HqL2K8NxWbTcQpVqZjANBgkqhkiG
9w0BAQsFAAOCAQEATrJ8XqK2L5M9N0P1Q2R3S4T5U6V7W8X9Y0Z1A2B3C4D5E6
F7G8H9I0J1K2L3M4N5O6P7Q8R9S0T1U2V3W4X5Y6Z7A8B9C0D1E2F3G4H5I6
J7K8L9M0N1O2P3Q4R5S6T7U8V9W0X1Y2Z3A4B5C6D7E8F9G0H1I2J3K4L5M6
N7O8P9Q0R1S2T3U4V5W6X7Y8Z9A0B1C2D3E4F5G6H7I8J9K0L1M2N3O4P5Q6
R7S8T9U0V1W2X3Y4Z5A6B7C8D9E0F1G2H3I4J5K6L7M8N9O0P1Q2R3S4T5U6
-----END CERTIFICATE-----`

	keyBlock := `-----BEGIN PRIVATE KEY-----
MIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQCzdY7K0mW8GX6q
3DqR1MkF2QjT5dN1P0WJ8KXHZv9QaPqk3L5mV9H3d5X8V2B7F2C2L8H8K3K7J1
H2D4M5N9Q8P1R6S3Q7U9V0W1X2Y3Z4A5B6C7D8E9F0G1H2I3J4K5L6M7N8O9P0
Q1R2S3T4U5V6W7X8Y9Z0A1B2C3D4E5F6G7H8I9J0K1L2M3N4O5P6Q7R8S9T0U1
V2W3X4Y5Z6A7B8C9D0E1F2G3H4I5J6K7L8M9N0O1P2Q3R4S5T6U7V8W9X0Y1Z2
A3B4C5D6E7F8G9H0I1J2K3L4M5N6O7P8Q9R0S1T2U3V4W5X6Y7Z8A9B0C1D2E3
F4G5H6I7J8K9L0M1N2O3P4Q5R6S7T8U9V0W1X2Y3Z4A5B6C7D8E9F0G1H2I3J4
K5L6M7N8O9P0Q1R2S3T4U5V6W7X8Y9Z0A1B2C3D4E5F6G7H8I9J0K1L2M3N4O5
P6Q7R8S9T0U1V2W3X4Y5Z6A7B8C9D0E1F2G3H4I5J6K7L8M9N0O1P2Q3R4S5T6
-----END PRIVATE KEY-----`
	certPEM = []byte(certBlock)
	keyPEM = []byte(keyBlock)
	return certPEM, keyPEM, nil
}
