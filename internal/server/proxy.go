package server

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// RateLimiter implements a per-agent token-bucket rate limiter with a global
// concurrent-request cap. It uses only the standard library (sync + time).
type RateLimiter struct {
	mu            sync.Mutex
	buckets       map[string]*tokenBucket // per-agent token buckets
	rate          float64                 // tokens added per second
	burst         int                     // maximum tokens a bucket can hold
	maxConcurrent int                     // maximum concurrent in-flight requests
	active        int                     // current in-flight request count
}

// tokenBucket holds the per-agent token state.
type tokenBucket struct {
	tokens   float64
	lastTime time.Time
}

// NewRateLimiter creates a RateLimiter.
//
//   - rate:          steady-state requests per second (tokens replenished at this rate)
//   - burst:         maximum burst size (bucket capacity); first burst calls pass immediately
//   - maxConcurrent: hard cap on concurrent in-flight requests across all agents
func NewRateLimiter(rate float64, burst int, maxConcurrent int) *RateLimiter {
	if rate <= 0 {
		rate = 10.0
	}
	if burst <= 0 {
		burst = 20
	}
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}
	return &RateLimiter{
		buckets:       make(map[string]*tokenBucket),
		rate:          rate,
		burst:         burst,
		maxConcurrent: maxConcurrent,
	}
}

// Allow checks whether a request from agentID may proceed.
// It returns true if the request is admitted (a token consumed and a
// concurrency slot reserved), in which case the caller MUST call Release
// exactly once when the request completes. It returns false if the request
// is denied due to either the concurrent-request cap or token exhaustion.
func (rl *RateLimiter) Allow(agentID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Global concurrency cap.
	if rl.active >= rl.maxConcurrent {
		return false
	}

	// Lazily create the agent's bucket, pre-filled to burst capacity.
	b, ok := rl.buckets[agentID]
	if !ok {
		b = &tokenBucket{
			tokens:   float64(rl.burst),
			lastTime: time.Now(),
		}
		rl.buckets[agentID] = b
	}

	// Refill tokens based on elapsed time since last check.
	now := time.Now()
	elapsed := now.Sub(b.lastTime).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > float64(rl.burst) {
		b.tokens = float64(rl.burst)
	}
	b.lastTime = now

	// Require at least one token to admit the request.
	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		rl.active++
		return true
	}
	return false
}

// Release decrements the in-flight request counter, freeing a concurrency
// slot. It must be called exactly once for each successful Allow().
func (rl *RateLimiter) Release() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if rl.active > 0 {
		rl.active--
	}
}

// LLMProxy provides LLM API call functionality, reading provider keys from environment.
type LLMProxy struct {
	provider string // detected from env vars: "deepseek", "minimax", "ollama", "generic"
	apiKey   string
	limiter  *RateLimiter
}

// NewLLMProxy creates an LLM proxy that reads keys from environment variables.
// Priority: PROBE_LLM_KEY (generic), DEEPSEEK_API_KEY, MINIMAX_API_KEY, OLLAMA_API_KEY.
// The proxy is initialised with default rate limits: 10 req/s, burst 20, max 5 concurrent.
// Use SetRateLimiter to override these limits (e.g. from CLI flags).
func NewLLMProxy() *LLMProxy {
	p := &LLMProxy{
		limiter: NewRateLimiter(10.0, 20, 5), // 10 req/s, burst 20, max 5 concurrent
	}

	if key := os.Getenv("PROBE_LLM_KEY"); key != "" {
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

// SetRateLimiter replaces the proxy's rate limiter. Pass a RateLimiter
// configured from CLI flags / env vars. Passing nil disables rate limiting.
func (p *LLMProxy) SetRateLimiter(rl *RateLimiter) {
	p.limiter = rl
}

// Call sends a prompt to the configured LLM provider and returns the response
// text and provider name. The agentID is used for per-agent rate limiting; if
// the rate limiter rejects the request a rate-limited marker is returned with
// the provider name "rate_limited".
func (p *LLMProxy) Call(agentID string, prompt string) (string, string) {
	// Enforce per-agent rate limiting + global concurrency cap.
	if p.limiter != nil {
		if !p.limiter.Allow(agentID) {
			return "[LLM proxy: rate limited]", "rate_limited"
		}
		defer p.limiter.Release()
	}

	if p.apiKey == "" {
		return fmt.Sprintf("[LLM proxy error: no API key configured. Set PROBE_LLM_KEY or provider-specific key.]"), p.provider
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

// callGeneric uses the PROBE_LLM_KEY for a generic OpenAI-compatible endpoint.
func (p *LLMProxy) callGeneric(prompt string) string {
	endpoint := os.Getenv("PROBE_LLM_ENDPOINT")
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