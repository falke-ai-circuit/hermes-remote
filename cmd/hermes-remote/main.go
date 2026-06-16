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
	connectURL    = flag.String("connect", "", "WebSocket server URL to connect to (wss://host:port)")
	listenAddr    = flag.String("listen", "", "Address to listen on for incoming connections (:port)")
	mode          = flag.String("mode", "silent", "Agent mode: silent or interactive")
	token         = flag.String("token", "", "Authentication token")
	name          = flag.String("name", "hermes-remote", "Display name for the agent")
	maxRetries    = flag.Int("max-retries", 0, "Max reconnect attempts (0=infinite)")
	backoffMin    = flag.Duration("backoff-min", 1*time.Second, "Min backoff duration")
	backoffMax    = flag.Duration("backoff-max", 60*time.Second, "Max backoff duration")
	tokenFile     = flag.String("token-file", ".hermes-remote-token", "Path to persist the auth token so rotated tokens survive reconnects (empty = no persistence)")
	certPath      = flag.String("cert", "", "CA certificate file (PEM) for verifying the server's TLS cert on outbound wss:// connections")
	clientCertFile = flag.String("client-cert", "", "Client certificate file (PEM) for TLS mutual authentication (mTLS) on outbound wss:// connections")
	clientKeyFile  = flag.String("client-key", "", "Client key file (PEM) for TLS mutual authentication (mTLS) on outbound wss:// connections")
	certFile       = flag.String("cert-file", "", "TLS certificate file (PEM) for inbound server mode")
	keyFile        = flag.String("key-file", "", "TLS key file (PEM) for inbound server mode")
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

	// If a token-file is configured and no --token was given on the command
	// line, try to resume with a previously persisted (rotated) token. This
	// lets the agent reconnect with the latest rotated token after a restart.
	if tokenVal == "" && *tokenFile != "" {
		if persisted, err := agent.LoadPersistedToken(*tokenFile); err != nil {
			log.Printf("[agent] warning: could not read token file %s: %v", *tokenFile, err)
		} else if persisted != "" {
			tokenVal = persisted
			log.Printf("[agent] resumed with persisted token from %s (len=%d)", *tokenFile, len(tokenVal))
		}
	}

	if tokenVal != "" {
		log.Printf("[agent] token length=%d first_char=%c last_char=%c", len(tokenVal), tokenVal[0], tokenVal[len(tokenVal)-1])
	}

	cfg := agent.Config{
		Mode:           "outbound",
		URL:            *connectURL,
		Addr:           *listenAddr,
		Token:          tokenVal,
		CertPath:       *certPath,
		ClientCertFile: *clientCertFile,
		ClientKeyFile:  *clientKeyFile,
		CertFile:       *certFile,
		KeyFile:        *keyFile,
		Name:           *name,
		MaxRetries:     *maxRetries,
		BackoffMin:     *backoffMin,
		BackoffMax:     *backoffMax,
		TokenFile:      *tokenFile,
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
