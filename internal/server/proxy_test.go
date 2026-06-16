package server

import (
	"testing"
	"time"
)

// TestRateLimiterBurst verifies that burst calls are admitted and subsequent
// calls are denied until tokens refill.
func TestRateLimiterBurst(t *testing.T) {
	rl := NewRateLimiter(1.0, 3, 10) // burst 3, generous concurrency
	pass := 0
	for i := 0; i < 5; i++ {
		if rl.Allow("agentA") {
			pass++
		}
	}
	if pass != 3 {
		t.Fatalf("expected 3 passes from burst of 3, got %d", pass)
	}
	// Release all so concurrency doesn't interfere.
	for i := 0; i < pass; i++ {
		rl.Release()
	}
}

// TestRateLimiterConcurrent verifies the global concurrent-request cap.
func TestRateLimiterConcurrent(t *testing.T) {
	rl := NewRateLimiter(100.0, 100, 2) // plenty of tokens, max 2 concurrent
	p := 0
	for i := 0; i < 3; i++ {
		if rl.Allow("agentA") {
			p++
		}
	}
	if p != 2 {
		t.Fatalf("expected 2 passes (concurrency cap 2), got %d", p)
	}
	// 3rd denied, active still 2
	rl.Release() // active -> 1
	if !rl.Allow("agentA") {
		t.Fatal("expected Allow to pass after a Release freed a slot")
	}
	rl.Release()
	rl.Release()
}

// TestRateLimiterRefill verifies tokens replenish over time.
func TestRateLimiterRefill(t *testing.T) {
	rl := NewRateLimiter(2.0, 2, 10) // 2 tokens/sec, burst 2
	rl.Allow("a")
	rl.Allow("a") // tokens exhausted
	rl.Release()
	rl.Release() // active back to 0
	if rl.Allow("a") {
		t.Fatal("expected denial immediately after exhausting tokens")
	}
	time.Sleep(600 * time.Millisecond) // ~1.2 tokens accrued
	if !rl.Allow("a") {
		t.Fatal("expected Allow to pass after 600ms refill")
	}
	rl.Release()
}

// TestRateLimiterIsolation verifies per-agent bucket independence.
func TestRateLimiterIsolation(t *testing.T) {
	rl := NewRateLimiter(1.0, 1, 10) // burst 1 per agent
	if !rl.Allow("agentA") {
		t.Fatal("agentA first call should pass")
	}
	if rl.Allow("agentA") {
		t.Fatal("agentA second call should be denied (bucket empty)")
	}
	if !rl.Allow("agentB") {
		t.Fatal("agentB first call should pass (separate bucket)")
	}
	rl.Release()
	rl.Release()
}

// TestLLMProxyRateLimited verifies that the proxy enforces rate limits and
// returns the "rate_limited" provider marker when the limiter denies a call.
func TestLLMProxyRateLimited(t *testing.T) {
	p := NewLLMProxy()
	p.SetRateLimiter(NewRateLimiter(100.0, 1, 10)) // burst 1
	_, prov := p.Call("agentX", "hello")
	if prov == "rate_limited" {
		t.Fatalf("first call should not be rate-limited, got provider=%q", prov)
	}
	_, prov2 := p.Call("agentX", "hello")
	if prov2 != "rate_limited" {
		t.Fatalf("second call should be rate-limited, got provider=%q", prov2)
	}
}

// TestNewServerWithRateLimit verifies the server constructor wires the limiter.
func TestNewServerWithRateLimit(t *testing.T) {
	srv := NewServerWithRateLimit(":0", "tok", "", RateLimitConfig{
		RatePerSec:    42,
		Burst:         7,
		MaxConcurrent: 3,
	})
	if srv.rateLimit == nil {
		t.Fatal("expected server.rateLimit to be set")
	}
	if srv.proxy == nil || srv.proxy.limiter == nil {
		t.Fatal("expected proxy.limiter to be wired")
	}
	if srv.rateLimit.burst != 7 {
		t.Fatalf("expected burst=7, got %d", srv.rateLimit.burst)
	}
	if srv.rateLimit.maxConcurrent != 3 {
		t.Fatalf("expected maxConcurrent=3, got %d", srv.rateLimit.maxConcurrent)
	}
}