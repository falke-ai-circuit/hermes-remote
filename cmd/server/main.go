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

	srv := server.NewServerWithRateLimit(*addr, *token, *registryPath, server.RateLimitConfig{
		RatePerSec:    *rateLimit,
		Burst:         *rateBurst,
		MaxConcurrent: *maxConcurrent,
	})
	srv.SetTokenTTL(*tokenTTL)
	log.Printf("Starting hermes-remote server on %s (rate-limit=%.1f req/s, burst=%d, max-concurrent=%d, token-ttl=%v)", *addr, *rateLimit, *rateBurst, *maxConcurrent, *tokenTTL)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}