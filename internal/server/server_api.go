package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/falke-ai-circuit/hermes-remote/internal/protocol"
)

// handleAgentRoute dispatches /api/agent/{id}/... routes.
func (s *Server) handleAgentRoute(w http.ResponseWriter, r *http.Request) {
	if !s.checkAPIAuth(w, r) {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/agent/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 {
		http.Error(w, "agent ID required", http.StatusBadRequest)
		return
	}

	agentID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	// Special case: /api/agent/file/{filename} serves files from the download directory.
	// This routes file downloads through the /api/agent/ prefix which Docker proxies forward.
	if agentID == "file" && action != "" {
		s.handleFileDownload(w, r)
		return
	}

	// Special case: /api/agent/{any-id}/download/{filename} serves files from the download directory.
	// This works through Docker proxies that only forward /api/agent/{known-id}/* paths.
	if action != "" && strings.HasPrefix(action, "download/") {
		s.handleFileDownload(w, r)
		return
	}

	// Special case: /api/agent/{any-id}/file-download serves a file specified in the JSON body.
	// This is a 2-level path that works through Docker proxies that only forward 2-level paths.
	if action == "file-download" && r.Method == http.MethodPost {
		s.handleFileDownloadBody(w, r)
		return
	}

	switch {
	case action == "exec" && r.Method == http.MethodPost:
		s.handleAgentExec(w, r, agentID)
	case action == "fs-read" && r.Method == http.MethodPost:
		s.handleAgentFSRead(w, r, agentID)
	case action == "fs-write" && r.Method == http.MethodPost:
		s.handleAgentFSWrite(w, r, agentID)
	case action == "capture" && r.Method == http.MethodPost:
		s.handleAgentCapture(w, r, agentID)
	case action == "task-list" && r.Method == http.MethodPost:
		s.handleAgentTaskList(w, r, agentID)
	case action == "proc-list" && r.Method == http.MethodPost:
		s.handleAgentProcList(w, r, agentID)
	case action == "proc-kill" && r.Method == http.MethodPost:
		s.handleAgentProcKill(w, r, agentID)
	case action == "proc-start" && r.Method == http.MethodPost:
		s.handleAgentProcStart(w, r, agentID)
	case action == "tunnel" && r.Method == http.MethodPost:
		s.handleAgentTunnel(w, r, agentID)
	case action == "tunnel-close" && r.Method == http.MethodPost:
		s.handleAgentTunnelClose(w, r, agentID)
	case action == "sniff" && r.Method == http.MethodPost:
		s.handleAgentSniff(w, r, agentID)
	case action == "sniff-stop" && r.Method == http.MethodPost:
		s.handleAgentSniffStop(w, r, agentID)
	case action == "mitm-start" && r.Method == http.MethodPost:
		s.handleAgentMitmStart(w, r, agentID)
	case action == "mitm-stop" && r.Method == http.MethodPost:
		s.handleAgentMitmStop(w, r, agentID)
	case action == "mitm-traffic" && r.Method == http.MethodPost:
		s.handleAgentMitmTraffic(w, r, agentID)
	case action == "debug-attach" && r.Method == http.MethodPost:
		s.handleAgentDebugAttach(w, r, agentID)
	case action == "debug-detach" && r.Method == http.MethodPost:
		s.handleAgentDebugDetach(w, r, agentID)
	case action == "debug-read-mem" && r.Method == http.MethodPost:
		s.handleAgentDebugReadMem(w, r, agentID)
	case action == "debug-modules" && r.Method == http.MethodPost:
		s.handleAgentDebugModules(w, r, agentID)
	case action == "debug-mem-query" && r.Method == http.MethodPost:
		s.handleAgentDebugMemQuery(w, r, agentID)
	case action == "health" && r.Method == http.MethodGet:
		s.handleAgentHealth(w, r, agentID)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// handleListAgents returns the list of registered agents as JSON.
func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	if !s.checkAPIAuth(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	agents := s.registry.ListAgents()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agents)
}


// handleAgentRoute dispatches /api/agent/{id}/... routes.


// handleAgentExec executes a shell command ON THE REMOTE AGENT via WebSocket.
// It forwards the command to the connected agent and waits for the response.
func (s *Server) handleAgentExec(w http.ResponseWriter, r *http.Request, agentID string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var params protocol.ExecParams
	if err := json.Unmarshal(body, &params); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if params.Timeout <= 0 {
		params.Timeout = 60
	}

	// Get the agent's WebSocket connection
	s.mu.RLock()
	conn, ok := s.conns[agentID]
	s.mu.RUnlock()
	if !ok {
		http.Error(w, fmt.Sprintf("agent %s not connected", agentID), http.StatusServiceUnavailable)
		return
	}

	// Build the command envelope
	reqID := fmt.Sprintf("exec-%d", time.Now().UnixMilli())
	paramData, _ := json.Marshal(params)
	env := protocol.Envelope{
		ID:     reqID,
		Type:   protocol.TypeExec,
		Params: paramData,
	}

	// Register a pending response channel
	respCh := make(chan protocol.Envelope, 1)
	s.pendingMu.Lock()
	s.pendingReqs[reqID] = respCh
	s.pendingMu.Unlock()
	defer func() {
		s.pendingMu.Lock()
		delete(s.pendingReqs, reqID)
		s.pendingMu.Unlock()
	}()

	// Send the command to the agent
	if err := conn.WriteJSON(env); err != nil {
		http.Error(w, fmt.Sprintf("send to agent failed: %v", err), http.StatusServiceUnavailable)
		return
	}

	// Wait for response with timeout
	select {
	case resp := <-respCh:
		if resp.Error != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"stdout":     "",
				"stderr":     resp.Error.Message,
				"exit_code":  -1,
				"duration_ms": 0,
				"timed_out":  false,
			})
			return
		}
		// Parse the exec result from the response
		var execResult protocol.ExecResult
		if resp.Result != nil {
			json.Unmarshal(resp.Result, &execResult)
		}
		s.sessions.AddMemory(agentID, "last_exec", params.Command)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(execResult)

	case <-time.After(time.Duration(params.Timeout) * time.Second):
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"stdout":     "",
			"stderr":     "command timed out",
			"exit_code":  -1,
			"duration_ms": params.Timeout * 1000,
			"timed_out":  true,
		})
	}
}


func mustMarshalRaw(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}


// handleAgentFSRead reads a file ON THE REMOTE AGENT via WebSocket.
func (s *Server) handleAgentFSRead(w http.ResponseWriter, r *http.Request, agentID string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var params protocol.FSParams
	if err := json.Unmarshal(body, &params); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.forwardToAgent(agentID, protocol.TypeFSRead, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}


// handleAgentFSWrite writes a file ON THE REMOTE AGENT via WebSocket.
func (s *Server) handleAgentFSWrite(w http.ResponseWriter, r *http.Request, agentID string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var params protocol.FSParams
	if err := json.Unmarshal(body, &params); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.forwardToAgent(agentID, protocol.TypeFileSave, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}


// handleAgentCapture captures display ON THE REMOTE AGENT via WebSocket.
func (s *Server) handleAgentCapture(w http.ResponseWriter, r *http.Request, agentID string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var params protocol.ScreenParams
	if err := json.Unmarshal(body, &params); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.forwardToAgent(agentID, protocol.TypeCapture, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}


// handleAgentTaskList lists processes ON THE REMOTE AGENT via WebSocket.
func (s *Server) handleAgentTaskList(w http.ResponseWriter, r *http.Request, agentID string) {
	resp, err := s.forwardToAgent(agentID, protocol.TypeTaskList, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}


// handleAgentProcList lists processes ON THE REMOTE AGENT via WebSocket (new).
func (s *Server) handleAgentProcList(w http.ResponseWriter, r *http.Request, agentID string) {
	resp, err := s.forwardToAgent(agentID, protocol.TypeProcList, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}


// handleAgentProcKill kills a process ON THE REMOTE AGENT via WebSocket.
func (s *Server) handleAgentProcKill(w http.ResponseWriter, r *http.Request, agentID string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var params protocol.TaskStopParams
	if err := json.Unmarshal(body, &params); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.forwardToAgent(agentID, protocol.TypeProcKill, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}


// handleAgentProcStart starts a process ON THE REMOTE AGENT via WebSocket.
func (s *Server) handleAgentProcStart(w http.ResponseWriter, r *http.Request, agentID string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var params protocol.ProcStartParams
	if err := json.Unmarshal(body, &params); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.forwardToAgent(agentID, protocol.TypeProcStart, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}


// forwardToAgent sends a command to the agent via WebSocket and waits for response.
// This is the generic request-response pattern used by all forwarded handlers.
func (s *Server) forwardToAgent(agentID string, msgType string, params interface{}) (interface{}, error) {
	s.mu.RLock()
	conn, ok := s.conns[agentID]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("agent %s not connected", agentID)
	}

	reqID := fmt.Sprintf("%s-%d", msgType, time.Now().UnixMilli())
	var paramData json.RawMessage
	if params != nil {
		paramData, _ = json.Marshal(params)
	}
	env := protocol.Envelope{
		ID:     reqID,
		Type:   msgType,
		Params: paramData,
	}

	respCh := make(chan protocol.Envelope, 1)
	s.pendingMu.Lock()
	s.pendingReqs[reqID] = respCh
	s.pendingMu.Unlock()
	defer func() {
		s.pendingMu.Lock()
		delete(s.pendingReqs, reqID)
		s.pendingMu.Unlock()
	}()

	if err := conn.WriteJSON(env); err != nil {
		return nil, fmt.Errorf("send to agent failed: %w", err)
	}

	select {
	case resp := <-respCh:
		if resp.Error != nil {
			return map[string]interface{}{
				"error": resp.Error.Message,
			}, nil
		}
		var result interface{}
		if resp.Result != nil {
			json.Unmarshal(resp.Result, &result)
		}
		return result, nil
	case <-time.After(120 * time.Second):
		return nil, fmt.Errorf("agent did not respond within 120s")
	}
}


// handleAgentHealth returns the full health record for a single agent.
func (s *Server) handleAgentHealth(w http.ResponseWriter, r *http.Request, agentID string) {
	rec, err := s.registry.GetHealth(agentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rec)
}

