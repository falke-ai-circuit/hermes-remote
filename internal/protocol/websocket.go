package protocol

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// Dial connects to a remote WebSocket server.
func Dial(rawURL string, certPath string, token string) (*websocket.Conn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	dialer := &websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
		},
	}

	if certPath != "" {
		certPool := x509.NewCertPool()
		certPEM, err := os.ReadFile(certPath)
		if err != nil {
			return nil, fmt.Errorf("read cert: %w", err)
		}
		if !certPool.AppendCertsFromPEM(certPEM) {
			return nil, fmt.Errorf("failed to parse cert")
		}
		dialer.TLSClientConfig.RootCAs = certPool
	} else {
		dialer.TLSClientConfig.InsecureSkipVerify = true
	}

	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	conn, _, err := dialer.Dial(u.String(), header)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	return conn, nil
}

// Listen starts a WebSocket server.
func Listen(addr string, certFile string, keyFile string) (*http.Server, *http.ServeMux, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
	}

	if certFile != "" {
		fingerprint, err := certFingerprint(certFile)
		if err != nil {
			return nil, nil, fmt.Errorf("cert fingerprint: %w", err)
		}
		fmt.Fprintf(os.Stderr, "TLS Certificate SHA256: %s\n", fingerprint)
	}

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:      addr,
		TLSConfig: tlsConfig,
		Handler:   mux,
	}

	return server, mux, nil
}

func certFingerprint(certFile string) (string, error) {
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return "", err
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return "", fmt.Errorf("failed to parse certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(cert.Raw)), nil
}

func WriteMessage(conn *websocket.Conn, v interface{}) error {
	return conn.WriteJSON(v)
}

func ReadMessage(conn *websocket.Conn, v interface{}) error {
	return conn.ReadJSON(v)
}

func ReadRawMessage(conn *websocket.Conn) (json.RawMessage, error) {
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	return json.RawMessage(msg), nil
}

func GetUpgrader() *websocket.Upgrader {
	return &upgrader
}
