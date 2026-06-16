package main

import (
	"flag"
	"log"
	"os"

	"github.com/falke-ai-circuit/hermes-remote/internal/server"
)

func main() {
	addr := flag.String("addr", "", "listen address (default: HERMES_REMOTE_ADDR env or localhost:7700)")
	token := flag.String("token", "", "auth token (default: HERMES_REMOTE_TOKEN env)")
	registryPath := flag.String("registry", "", "registry file path (default: HERMES_REMOTE_REGISTRY env or /tmp/hermes-remote-registry.json)")
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

	srv := server.NewServer(*addr, *token, *registryPath)
	log.Printf("Starting hermes-remote server on %s", *addr)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
