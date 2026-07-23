package features

// APIHandler provides HTTP REST API endpoints for the diagnostic dashboard.
// These endpoints expose system information, health checks, and metrics
// in a JSON format suitable for monitoring dashboards.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// APIHandler manages HTTP API endpoints.
type APIHandler struct {
	metrics   *MetricsCollector
	monitor   *HealthMonitor
	scheduler *Scheduler
	logger    *Logger
}

// NewAPIHandler creates a new API handler with the given components.
func NewAPIHandler(metrics *MetricsCollector, monitor *HealthMonitor, scheduler *Scheduler) *APIHandler {
	return &APIHandler{
		metrics:   metrics,
		monitor:   monitor,
		scheduler: scheduler,
	}
}

// RegisterRoutes registers all API routes on the given mux.
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/health", h.handleHealth)
	mux.HandleFunc("/api/v1/metrics", h.handleMetrics)
	mux.HandleFunc("/api/v1/system", h.handleSystemInfo)
	mux.HandleFunc("/api/v1/tasks", h.handleTasks)
	mux.HandleFunc("/api/v1/version", h.handleVersion)
	mux.HandleFunc("/api/v1/status", h.handleStatus)
}

// handleHealth returns the current health status as JSON.
func (h *APIHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	status := h.monitor.RunChecks()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleMetrics returns all collected metrics.
func (h *APIHandler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	metrics := h.metrics.GetAllMetrics()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// handleSystemInfo returns system diagnostic information.
func (h *APIHandler) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	info, err := CollectSystemInfo()
	if err != nil {
		http.Error(w, fmt.Sprintf("collect system info: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// handleTasks returns scheduled task information.
func (h *APIHandler) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	tasks := h.scheduler.GetTaskInfo()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

// handleVersion returns the application version information.
func (h *APIHandler) handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"version":   "1.0.0",
		"buildDate": time.Now().Format(time.RFC3339),
		"component": "PROBE Client",
	})
}

// handleStatus returns a summary of all system components.
func (h *APIHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	info, _ := CollectSystemInfo()
	health := h.monitor.LastStatus()
	tasks := h.scheduler.GetTaskInfo()
	
	summary := map[string]interface{}{
		"system":      info,
		"health":      health,
		"tasks":       tasks,
		"uptime":      h.monitor.Uptime().String(),
		"status":      "operational",
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

// WriteJSON writes a JSON response with proper headers.
func WriteJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, fmt.Sprintf("encode response: %v", err), http.StatusInternalServerError)
	}
}

// ParseAPIPath extracts the resource name from an API path.
func ParseAPIPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}
