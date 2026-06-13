package server

import (
	"sync"
)

// SessionContext holds per-agent session data: memories, skills, context.
type SessionContext struct {
	AgentID  string
	Memories []Memory
	Skills   []Skill
	Context  string // arbitrary context string for the Hermes session
}

// Memory is a key-value memory entry.
type Memory struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Skill is a named capability the agent can invoke.
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

// SessionManager manages per-agent Hermes session contexts.
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*SessionContext
}

// NewSessionManager creates a new session manager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*SessionContext),
	}
}

// CreateSession creates a new session context for an agent.
func (sm *SessionManager) CreateSession(agentID string) *SessionContext {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	ctx := &SessionContext{
		AgentID:  agentID,
		Memories: make([]Memory, 0),
		Skills:   make([]Skill, 0),
		Context:  "",
	}
	sm.sessions[agentID] = ctx
	return ctx
}

// GetSession retrieves the session for an agent, or nil if not found.
func (sm *SessionManager) GetSession(agentID string) *SessionContext {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.sessions[agentID]
}

// RemoveSession deletes an agent's session.
func (sm *SessionManager) RemoveSession(agentID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, agentID)
}

// AddMemory adds a memory entry to an agent's session.
func (sm *SessionManager) AddMemory(agentID, key, value string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if ctx, ok := sm.sessions[agentID]; ok {
		ctx.Memories = append(ctx.Memories, Memory{Key: key, Value: value})
	}
}

// AddSkill adds a skill to an agent's session.
func (sm *SessionManager) AddSkill(agentID, name, description string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if ctx, ok := sm.sessions[agentID]; ok {
		ctx.Skills = append(ctx.Skills, Skill{Name: name, Description: description, Enabled: true})
	}
}

// SetContext sets the context string for an agent's session.
func (sm *SessionManager) SetContext(agentID, context string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if ctx, ok := sm.sessions[agentID]; ok {
		ctx.Context = context
	}
}

// ListSessions returns a copy of all active session IDs.
func (sm *SessionManager) ListSessions() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	ids := make([]string, 0, len(sm.sessions))
	for id := range sm.sessions {
		ids = append(ids, id)
	}
	return ids
}
