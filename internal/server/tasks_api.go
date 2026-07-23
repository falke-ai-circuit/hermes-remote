package server

import (
	"encoding/json"
	"net/http"
)

// ---------------------------------------------------------------------------
// Task API endpoints (Phase 5)
// ---------------------------------------------------------------------------

// handleV1CreateTask creates a new scheduled task.
// POST /api/v1/tasks
func (s *Server) handleV1CreateTask(w http.ResponseWriter, r *http.Request) {
	op, ok := s.v1CheckAuth(w, r, "exec")
	if !ok {
		return
	}

	var req struct {
		AgentID     string          `json:"agent_id"`
		CommandType string          `json:"command_type"`
		Params      json.RawMessage `json:"params"`
		Schedule    TaskSchedule    `json:"schedule"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON: "+err.Error())
		return
	}

	operatorID := ""
	if op != nil {
		operatorID = op.ID
	}

	task, err := s.tasks.Create(req.AgentID, req.CommandType, req.Params, req.Schedule, operatorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

// handleV1ListTasks lists tasks with optional filters.
// GET /api/v1/tasks?agent_id=X&status=pending
func (s *Server) handleV1ListTasks(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.v1CheckAuth(w, r, "list"); !ok {
		return
	}
	filter := TaskFilter{
		AgentID: r.URL.Query().Get("agent_id"),
		Status:  r.URL.Query().Get("status"),
	}
	tasks := s.tasks.List(filter)
	writeJSON(w, http.StatusOK, tasks)
}

// handleV1GetTask returns a single task's status and result.
// GET /api/v1/tasks/{id}
func (s *Server) handleV1GetTask(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.v1CheckAuth(w, r, "list"); !ok {
		return
	}
	taskID := r.PathValue("id")
	task, err := s.tasks.Get(taskID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, task)
}

// handleV1CancelTask cancels a pending or running task.
// DELETE /api/v1/tasks/{id}
func (s *Server) handleV1CancelTask(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.v1CheckAuth(w, r, "exec"); !ok {
		return
	}
	taskID := r.PathValue("id")
	if err := s.tasks.Cancel(taskID); err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"cancelled": taskID})
}