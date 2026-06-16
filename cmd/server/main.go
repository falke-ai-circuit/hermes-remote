package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/falke-ai-circuit/hermes-remote/internal/server"
)

func main() {
	addr := flag.String("addr", "", "listen address (default: HERMES_REMOTE_ADDR env or localhost:7700)")
	token := flag.String("token", "", "auth token (default: HERMES_REMOTE_TOKEN env)")
	registryPath := flag.String("registry", "", "registry file path (default: HERMES_REMOTE_REGISTRY env or /tmp/hermes-remote-registry.json)")
	rateLimit := flag.Float64("rate-limit", 10.0, "LLM proxy rate limit: requests per second per agent (default 10)")
	rateBurst := flag.Int("rate-burst", 20, "LLM proxy burst size: max tokens accumulated per agent (default 20)")
	maxConcurrent := flag.Int("max-concurrent", 5, "LLM proxy max concurrent in-flight requests across all agents (default 5)")
	tokenTTL := flag.Duration("token-ttl", 24*time.Hour, "token rotation interval; the server proactively rotates each agent's token before it expires. 0 disables rotation.")
	certFile := flag.String("cert-file", "", "TLS certificate file (PEM) for the server; enables TLS when provided with --key-file")
	keyFile := flag.String("key-file", "", "TLS key file (PEM) for the server; enables TLS when provided with --cert-file")
	clientCA := flag.String("client-ca", "", "Client CA certificate file (PEM) for TLS mutual authentication; requires --cert-file/--key-file")
	flag.Parse()

	// Env vars as fallback
	if *addr == "" {
		*addr = os.Getenv("HERMES_REMOTE_ADDR")
	}
	if *addr == "" {
		*addr = "localhost:7700"
	}
	if *token == "" {
		*token = os.Getenv("HERMES_REMOTE_TOKEN")
	}
	if *registryPath == "" {
		*registryPath = os.Getenv("HERMES_REMOTE_REGISTRY")
	}
	if *registryPath == "" {
		*registryPath = "/tmp/hermes-remote-registry.json"
	}

	rlCfg := server.RateLimitConfig{
		RatePerSec:    *rateLimit,
		Burst:         *rateBurst,
		MaxConcurrent: *maxConcurrent,
	}

	// TLS mode: when a cert and key are provided, start with TLS. When a
	// client CA is also provided, enable TLS mutual authentication (mTLS).
	useTLS := *certFile != "" && *keyFile != ""
	if useTLS && *clientCA != "" {
		log.Printf("Starting hermes-remote server with TLS+mTLS on %s (rate-limit=%.1f req/s, burst=%d, max-concurrent=%d, token-ttl=%v, client-ca=%s)", *addr, *rateLimit, *rateBurst, *maxConcurrent, *tokenTTL, *clientCA)
	} else if useTLS {
		log.Printf("Starting hermes-remote server with TLS on %s (rate-limit=%.1f req/s, burst=%d, max-concurrent=%d, token-ttl=%v)", *addr, *rateLimit, *rateBurst, *maxConcurrent, *tokenTTL)
	} else {
		log.Printf("Starting hermes-remote server on %s (rate-limit=%.1f req/s, burst=%d, max-concurrent=%d, token-ttl=%v)", *addr, *rateLimit, *rateBurst, *maxConcurrent, *tokenTTL)
	}

	var srv *server.Server
	if useTLS {
		srv = server.NewServerWithTLSRateLimit(*addr, *token, *registryPath, *certFile, *keyFile, *clientCA, rlCfg)
	} else {
		srv = server.NewServerWithRateLimit(*addr, *token, *registryPath, rlCfg)
	}
	srv.SetTokenTTL(*tokenTTL)

	if useTLS {
		if err := srv.StartTLS("", ""); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	} else {
		if err := srv.Start(); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}
}