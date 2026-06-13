package main

import (
	"log"
	"os"

	"github.com/falke-ai-circuit/hermes-remote/internal/server"
)

func main() {
	addr := os.Getenv("HERMES_REMOTE_ADDR")
	if addr == "" {
		addr = "localhost:7700"
	}
	token := os.Getenv("HERMES_REMOTE_TOKEN")
	registryPath := os.Getenv("HERMES_REMOTE_REGISTRY")
	if registryPath == "" {
		registryPath = "/tmp/hermes-remote-registry.json"
	}

	srv := server.NewServer(addr, token, registryPath)
	log.Printf("Starting hermes-remote server on %s", addr)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
