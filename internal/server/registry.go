package server

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// AgentRecord represents a registered agent.
type AgentRecord struct {
	AgentID       string `json:"agent_id"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	Mode          string `json:"mode"`
	ConnectedAt   string `json:"connected_at"`
	LastHeartbeat string `json:"last_heartbeat"`
	Status        string `json:"status"` // "active" or "inactive"
}

// Registry holds connected agents and persists them to disk.
type Registry struct {
	mu       sync.RWMutex
	agents   map[string]*AgentRecord
	savePath string
}

// NewRegistry creates a new agent registry.
func NewRegistry(savePath string) *Registry {
	r := &Registry{
		agents:   make(map[string]*AgentRecord),
		savePath: savePath,
	}
	r.load()
	return r
}

// Register adds or updates an agent record.
func (r *Registry) Register(agentID, name, version, goos, arch, mode string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	if rec, ok := r.agents[agentID]; ok {
		rec.Name = name
		rec.Version = version
		rec.OS = goos
		rec.Arch = arch
		rec.Mode = mode
		rec.LastHeartbeat = now
		rec.Status = "active"
	} else {
		r.agents[agentID] = &AgentRecord{
			AgentID:       agentID,
			Name:          name,
			Version:       version,
			OS:            goos,
			Arch:          arch,
			Mode:          mode,
			ConnectedAt:   now,
			LastHeartbeat: now,
			Status:        "active",
		}
	}
	r.save()
}

// Unregister marks an agent as inactive.
func (r *Registry) Unregister(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rec, ok := r.agents[agentID]; ok {
		rec.Status = "inactive"
		rec.LastHeartbeat = time.Now().UTC().Format(time.RFC3339)
		r.save()
	}
}

// ListAgents returns a slice of all agent records.
func (r *Registry) ListAgents() []AgentRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]AgentRecord, 0, len(r.agents))
	for _, rec := range r.agents {
		result = append(result, *rec)
	}
	return result
}

// Heartbeat updates the last heartbeat timestamp for an agent.
func (r *Registry) Heartbeat(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rec, ok := r.agents[agentID]; ok {
		rec.LastHeartbeat = time.Now().UTC().Format(time.RFC3339)
		rec.Status = "active"
		r.save()
	}
}

// save persists the registry to disk.
func (r *Registry) save() {
	if r.savePath == "" {
		return
	}
	dir := "/"
	for i := len(r.savePath) - 1; i >= 0; i-- {
		if r.savePath[i] == '/' {
			dir = r.savePath[:i]
			break
		}
	}
	if dir != "" && dir != "/" {
		os.MkdirAll(dir, 0755)
	}
	data, err := json.MarshalIndent(r.agents, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[registry] save marshal error: %v\n", err)
		return
	}
	if err := os.WriteFile(r.savePath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "[registry] save write error: %v\n", err)
	}
}

// load reads the registry from disk.
func (r *Registry) load() {
	data, err := os.ReadFile(r.savePath)
	if err != nil {
		return // file doesn't exist yet, that's fine
	}
	var agents map[string]*AgentRecord
	if err := json.Unmarshal(data, &agents); err != nil {
		fmt.Fprintf(os.Stderr, "[registry] load unmarshal error: %v\n", err)
		return
	}
	r.agents = agents
}
