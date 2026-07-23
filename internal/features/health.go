package features

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

type HealthStatus struct {
	Status    string        `json:"status"`
	Timestamp time.Time     `json:"timestamp"`
	Checks    []HealthCheck `json:"checks"`
	Uptime    time.Duration `json:"uptime"`
}

type HealthCheck struct {
	Name      string        `json:"name"`
	Status    string        `json:"status"`
	Message   string        `json:"message"`
	Duration  time.Duration `json:"duration"`
	Threshold float64       `json:"threshold"`
	Value     float64       `json:"value"`
}

type HealthMonitor struct {
	mu          sync.Mutex
	checks       []CheckFunc
	interval     time.Duration
	lastStatus   *HealthStatus
	startTime    time.Time
	alertHandler func(check HealthCheck)
}

type CheckFunc func() HealthCheck

func NewHealthMonitor(interval time.Duration) *HealthMonitor {
	return &HealthMonitor{interval: interval, startTime: time.Now(), checks: make([]CheckFunc, 0)}
}

func (m *HealthMonitor) RegisterCheck(name string, check func() (float64, error), threshold float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checks = append(m.checks, func() HealthCheck {
		start := time.Now()
		value, err := check()
		duration := time.Since(start)
		status := "pass"
		msg := "OK"
		if err != nil {
			status = "fail"
			msg = err.Error()
		} else if value > threshold {
			status = "warn"
			msg = fmt.Sprintf("%.2f exceeds threshold %.2f", value, threshold)
		}
		return HealthCheck{Name: name, Status: status, Message: msg, Duration: duration, Threshold: threshold, Value: value}
	})
}

func (m *HealthMonitor) RunChecks() *HealthStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	status := &HealthStatus{Timestamp: time.Now(), Uptime: time.Since(m.startTime), Checks: make([]HealthCheck, 0, len(m.checks))}
	overall := "healthy"
	for _, check := range m.checks {
		result := check()
		status.Checks = append(status.Checks, result)
		if result.Status == "fail" { overall = "critical" } else if result.Status == "warn" && overall != "critical" { overall = "warning" }
		if m.alertHandler != nil && (result.Status == "warn" || result.Status == "fail") { m.alertHandler(result) }
	}
	status.Status = overall
	m.lastStatus = status
	return status
}

func (m *HealthMonitor) SetAlertHandler(handler func(check HealthCheck)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertHandler = handler
}

func (m *HealthMonitor) LastStatus() *HealthStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastStatus
}

func (m *HealthMonitor) Uptime() time.Duration {
	return time.Since(m.startTime)
}

func MemoryUsageCheck(thresholdMB float64) CheckFunc {
	return func() HealthCheck {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		usageMB := float64(m.Alloc) / 1024 / 1024
		status := "pass"
		if usageMB > thresholdMB { status = "warn" }
		return HealthCheck{Name: "memory_usage", Status: status, Message: fmt.Sprintf("%.2f MB allocated", usageMB), Value: usageMB, Threshold: thresholdMB}
	}
}

func GoroutineCountCheck(threshold int) CheckFunc {
	return func() HealthCheck {
		count := runtime.NumGoroutine()
		status := "pass"
		if count > threshold { status = "warn" }
		return HealthCheck{Name: "goroutine_count", Status: status, Message: fmt.Sprintf("%d goroutines running", count), Value: float64(count), Threshold: float64(threshold)}
	}
}
