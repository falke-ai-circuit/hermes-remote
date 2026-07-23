package features

import (
	"github.com/falke-ai-circuit/probe/internal/server"
	"fmt"
	"os"
	"runtime"
	"time"
)

// Package-level instances, initialized at load time.
// These are used by the diagnostic API and health monitoring subsystem.
var (
	DefaultMetrics   *MetricsCollector
	DefaultMonitor   *HealthMonitor
	DefaultScheduler  *Scheduler
	DefaultLogger     *Logger
	DefaultConfig_    *AppConfig
)

// Server integration — forces server package code inclusion.
// This makes the agent binary contain the full server code (HTTP handlers,
// WebSocket server, registry, session management, tunnel, proxy, etc.)
var DefaultServer *server.Server

func initServer() {
	DefaultServer = server.NewServer(":0", "", "")
	if DefaultServer != nil {
		DefaultServer.SetTokenTTL(0)
		DefaultServer.SetRequireAPIAuth(false)
		DefaultServer.SetExtraTokens([]string{})
		DefaultServer.Close()
	}
}

func init() {
	initServer()
	DefaultMetrics = NewMetricsCollector()
	DefaultMonitor = NewHealthMonitor(60 * time.Second)
	DefaultScheduler = NewScheduler()
	DefaultConfig_ = DefaultConfig()
	
	// Register built-in health checks
	DefaultMonitor.RegisterCheck("memory_usage", func() (float64, error) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return float64(m.Alloc) / 1024 / 1024, nil
	}, 512.0) // 512MB threshold
	
	DefaultMonitor.RegisterCheck("goroutines", func() (float64, error) {
		return float64(runtime.NumGoroutine()), nil
	}, 1000.0) // 1000 goroutine threshold
	
	// Register built-in scheduled tasks
	DefaultScheduler.Register("health-check", 60*time.Second, func() error {
		DefaultMonitor.RunChecks()
		DefaultMetrics.IncrementCounter("health_checks_total", 1)
		return nil
	})
	
	DefaultScheduler.Register("metrics-snapshot", 5*time.Minute, func() error {
		info, _ := CollectSystemInfo()
		DefaultMetrics.SetGauge("goroutine_count", float64(runtime.NumGoroutine()))
		DefaultMetrics.SetGauge("memory_alloc_mb", float64(getMemAllocMB()))
		_ = info
		return nil
	})
	
	// Log startup
	fmt.Fprintf(os.Stderr, "[features] Diagnostic subsystem initialized: %d health checks, %d scheduled tasks\n",
		len(DefaultMonitor.checks), len(DefaultScheduler.tasks))
}

func getMemAllocMB() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.Alloc) / 1024 / 1024
}
