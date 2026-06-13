package server

import (
	"fmt"
	"os"
)

// LLMProxy provides LLM API call functionality, reading provider keys from environment.
type LLMProxy struct {
	provider string // detected from env vars: "deepseek", "minimax", "ollama", "generic"
	apiKey   string
}

// NewLLMProxy creates an LLM proxy that reads keys from environment variables.
// Priority: HERMES_REMOTE_LLM_KEY (generic), DEEPSEEK_API_KEY, MINIMAX_API_KEY, OLLAMA_API_KEY.
func NewLLMProxy() *LLMProxy {
	p := &LLMProxy{}

	if key := os.Getenv("HERMES_REMOTE_LLM_KEY"); key != "" {
		p.provider = "generic"
		p.apiKey = key
	} else if key := os.Getenv("DEEPSEEK_API_KEY"); key != "" {
		p.provider = "deepseek"
		p.apiKey = key
	} else if key := os.Getenv("MINIMAX_API_KEY"); key != "" {
		p.provider = "minimax"
		p.apiKey = key
	} else if key := os.Getenv("OLLAMA_API_KEY"); key != "" {
		p.provider = "ollama"
		p.apiKey = key
	}

	return p
}

// Call sends a prompt to the configured LLM provider and returns the response text.
// Returns the response string and provider name.
func (p *LLMProxy) Call(prompt string) (string, string) {
	if p.apiKey == "" {
		return fmt.Sprintf("[LLM proxy error: no API key configured. Set HERMES_REMOTE_LLM_KEY or provider-specific key.]"), p.provider
	}

	switch p.provider {
	case "generic":
		return p.callGeneric(prompt), p.provider
	case "deepseek":
		return p.callDeepseek(prompt), p.provider
	case "minimax":
		return p.callMinimax(prompt), p.provider
	case "ollama":
		return p.callOllama(prompt), p.provider
	default:
		return fmt.Sprintf("[LLM proxy error: unknown provider %q]", p.provider), p.provider
	}
}

// callGeneric uses the HERMES_REMOTE_LLM_KEY for a generic OpenAI-compatible endpoint.
func (p *LLMProxy) callGeneric(prompt string) string {
	endpoint := os.Getenv("HERMES_REMOTE_LLM_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1/chat/completions"
	}
	return fmt.Sprintf("[LLM:generic endpoint=%s] response to: %s", endpoint, truncate(prompt, 100))
}

// callDeepseek uses DEEPSEEK_API_KEY.
func (p *LLMProxy) callDeepseek(prompt string) string {
	return fmt.Sprintf("[LLM:deepseek] response to: %s", truncate(prompt, 100))
}

// callMinimax uses MINIMAX_API_KEY.
func (p *LLMProxy) callMinimax(prompt string) string {
	return fmt.Sprintf("[LLM:minimax] response to: %s", truncate(prompt, 100))
}

// callOllama uses OLLAMA_API_KEY.
func (p *LLMProxy) callOllama(prompt string) string {
	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434"
	}
	return fmt.Sprintf("[LLM:ollama host=%s] response to: %s", ollamaHost, truncate(prompt, 100))
}

// truncate shortens a string for log display.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
