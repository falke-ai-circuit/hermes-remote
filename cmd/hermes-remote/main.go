package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/falke-ai-circuit/hermes-remote/internal/agent"
)

var (
	connectURL = flag.String("connect", "", "WebSocket server URL to connect to (wss://host:port)")
	listenAddr = flag.String("listen", "", "Address to listen on for incoming connections (:port)")
	mode       = flag.String("mode", "silent", "Agent mode: silent or interactive")
	token      = flag.String("token", "", "Authentication token")
	name       = flag.String("name", "hermes-remote", "Display name for the agent")
	maxRetries = flag.Int("max-retries", 0, "Max reconnect attempts (0=infinite)")
	backoffMin = flag.Duration("backoff-min", 1*time.Second, "Min backoff duration")
	backoffMax = flag.Duration("backoff-max", 60*time.Second, "Max backoff duration")
)

func main() {
	flag.Parse()

	if *connectURL == "" && *listenAddr == "" {
		fmt.Fprintf(os.Stderr, "Usage: hermes-remote --connect wss://host:port [--mode silent|interactive] [--token TOKEN] [--name NAME]\n")
		fmt.Fprintf(os.Stderr, "       hermes-remote --listen :port [--mode silent|interactive] [--token TOKEN] [--name NAME]\n")
		os.Exit(1)
	}

	if *mode != "silent" && *mode != "interactive" {
		log.Fatalf("Invalid mode: %s (must be 'silent' or 'interactive')", *mode)
	}

	// Strip surrounding quotes from token (shell expansion can embed them)
	tokenVal := strings.Trim(*token, "\"'")
	if tokenVal != *token {
		log.Printf("[agent] stripped quotes from token (was %d chars, now %d chars)", len(*token), len(tokenVal))
	}
	if tokenVal != "" {
		log.Printf("[agent] token length=%d first_char=%c last_char=%c", len(tokenVal), tokenVal[0], tokenVal[len(tokenVal)-1])
	}

	cfg := agent.Config{
		Mode:       "outbound",
		URL:        *connectURL,
		Addr:       *listenAddr,
		Token:      tokenVal,
		Name:       *name,
		MaxRetries: *maxRetries,
		BackoffMin: *backoffMin,
		BackoffMax: *backoffMax,
	}

	// Ensure the WebSocket URL includes the /ws path the server expects
	if cfg.URL != "" && !strings.Contains(cfg.URL, "/ws") {
		cfg.URL = strings.TrimRight(cfg.URL, "/") + "/ws"
	}

	if *listenAddr != "" {
		cfg.Mode = "inbound"
	}

	ag := agent.New(cfg)

	// Handle SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start agent in background
	go func() {
		if err := ag.Run(); err != nil {
			log.Fatalf("Agent error: %v", err)
		}
	}()

	// Interactive mode: read stdin and send prompts to server
	if *mode == "interactive" {
		go runInteractive(ag)
	}

	<-sigCh
	log.Println("[cli] shutting down...")
}

func runInteractive(ag *agent.Agent) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Fprintf(os.Stderr, "[cli] Interactive mode. Type commands (or Ctrl+D to quit):\n")
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		ag.SendPrompt(line)
	}
	if err := scanner.Err(); err != nil {
		log.Printf("[cli] stdin error: %v", err)
	}
}
