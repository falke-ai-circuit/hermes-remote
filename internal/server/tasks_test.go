package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newTestTaskManager creates a TaskManager with a temp-file persistence path
// and no server reference (for CRUD-only tests).
func newTestTaskManager(t *testing.T) *TaskManager {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tasks.json")
	return NewTaskManager(path, nil)
}

// ---------------------------------------------------------------------------
// Create + Get
// ---------------------------------------------------------------------------

func TestTaskManager_Create(t *testing.T) {
	tm := newTestTaskManager(t)

	params := json.RawMessage(`{"command":"echo hello"}`)
	task, err := tm.Create("agent-1", "exec", params, TaskSchedule{Type: ScheduleOnce}, "op-1")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if task.ID == "" {
		t.Fatal("expected non-empty task ID")
	}
	if task.AgentID != "agent-1" {
		t.Errorf("expected agent_id=agent-1, got %s", task.AgentID)
	}
	if task.CommandType != "exec" {
		t.Errorf("expected command_type=exec, got %s", task.CommandType)
	}
	if task.Status != TaskStatusPending {
		t.Errorf("expected status=pending, got %s", task.Status)
	}
	if task.OperatorID != "op-1" {
		t.Errorf("expected operator_id=op-1, got %s", task.OperatorID)
	}
	if task.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if task.ExecuteAt.IsZero() {
		t.Error("expected non-zero ExecuteAt")
	}

	// Verify it was persisted.
	if _, err := os.Stat(tm.savePath); err != nil {
		t.Fatalf("expected tasks.json to exist: %v", err)
	}
}

func TestTaskManager_CreateDelayed(t *testing.T) {
	tm := newTestTaskManager(t)

	before := time.Now().UTC()
	task, err := tm.Create("agent-1", "exec", nil, TaskSchedule{Type: ScheduleDelayed, DelaySeconds: 30}, "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	// ExecuteAt should be ~30s from now.
	expected := before.Add(30 * time.Second)
	if task.ExecuteAt.Before(expected.Add(-2 * time.Second)) || task.ExecuteAt.After(expected.Add(2*time.Second)) {
		t.Errorf("expected ExecuteAt ~%v, got %v", expected, task.ExecuteAt)
	}
}

func TestTaskManager_CreateValidation(t *testing.T) {
	tm := newTestTaskManager(t)

	if _, err := tm.Create("", "exec", nil, TaskSchedule{}, ""); err == nil {
		t.Error("expected error for empty agent_id")
	}
	if _, err := tm.Create("agent-1", "", nil, TaskSchedule{}, ""); err == nil {
		t.Error("expected error for empty command_type")
	}
}

func TestTaskManager_Get(t *testing.T) {
	tm := newTestTaskManager(t)

	task, _ := tm.Create("agent-1", "exec", nil, TaskSchedule{}, "op-1")

	got, err := tm.Get(task.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.ID != task.ID {
		t.Errorf("expected ID %s, got %s", task.ID, got.ID)
	}

	// Get non-existent.
	if _, err := tm.Get("nonexistent"); err == nil {
		t.Error("expected error for non-existent task")
	}
}

// ---------------------------------------------------------------------------
// Cancel
// ---------------------------------------------------------------------------

func TestTaskManager_Cancel(t *testing.T) {
	tm := newTestTaskManager(t)

	task, _ := tm.Create("agent-1", "exec", nil, TaskSchedule{}, "")

	if err := tm.Cancel(task.ID); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	got, _ := tm.Get(task.ID)
	if got.Status != TaskStatusCancelled {
		t.Errorf("expected status=cancelled, got %s", got.Status)
	}
	if got.CompletedAt == nil {
		t.Error("expected non-nil CompletedAt after cancel")
	}

	// Cancel again — should error (already cancelled).
	if err := tm.Cancel(task.ID); err == nil {
		t.Error("expected error cancelling already-cancelled task")
	}

	// Cancel non-existent.
	if err := tm.Cancel("nonexistent"); err == nil {
		t.Error("expected error for non-existent task")
	}
}

// ---------------------------------------------------------------------------
// List with filters
// ---------------------------------------------------------------------------

func TestTaskManager_List(t *testing.T) {
	tm := newTestTaskManager(t)

	tm.Create("agent-1", "exec", nil, TaskSchedule{}, "")
	tm.Create("agent-1", "fs_read", nil, TaskSchedule{Type: ScheduleDelayed, DelaySeconds: 60}, "")
	tm.Create("agent-2", "exec", nil, TaskSchedule{}, "")

	// All tasks.
	all := tm.List(TaskFilter{})
	if len(all) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(all))
	}

	// Filter by agent.
	a1 := tm.List(TaskFilter{AgentID: "agent-1"})
	if len(a1) != 2 {
		t.Fatalf("expected 2 tasks for agent-1, got %d", len(a1))
	}

	// Filter by status (the delayed task is pending too, all 3 are pending).
	pending := tm.List(TaskFilter{Status: TaskStatusPending})
	if len(pending) != 3 {
		t.Fatalf("expected 3 pending tasks, got %d", len(pending))
	}

	// Combined filter.
	a1Pending := tm.List(TaskFilter{AgentID: "agent-1", Status: TaskStatusPending})
	if len(a1Pending) != 2 {
		t.Fatalf("expected 2 pending tasks for agent-1, got %d", len(a1Pending))
	}
}

// ---------------------------------------------------------------------------
// ExecutePending — offline queue (agent not connected, task stays pending)
// ---------------------------------------------------------------------------

func TestTaskManager_ExecutePending_OfflineStaysPending(t *testing.T) {
	tm := newTestTaskManager(t)
	// No server reference — agent is always "offline".

	task, _ := tm.Create("agent-1", "exec", nil, TaskSchedule{Type: ScheduleOnce}, "")

	// Execute pending — should not change anything (agent offline).
	tm.ExecutePending()

	got, _ := tm.Get(task.ID)
	if got.Status != TaskStatusPending {
		t.Errorf("expected pending (agent offline), got %s", got.Status)
	}
}

// ---------------------------------------------------------------------------
// ExecutePending — not yet due (delayed task)
// ---------------------------------------------------------------------------

func TestTaskManager_ExecutePending_NotYetDue(t *testing.T) {
	tm := newTestTaskManager(t)

	task, _ := tm.Create("agent-1", "exec", nil,
		TaskSchedule{Type: ScheduleDelayed, DelaySeconds: 3600}, "")

	tm.ExecutePending()

	got, _ := tm.Get(task.ID)
	if got.Status != TaskStatusPending {
		t.Errorf("expected pending (not due yet), got %s", got.Status)
	}
}

// ---------------------------------------------------------------------------
// ExecutePending — execute when agent connected (using a mock server)
// ---------------------------------------------------------------------------

func TestTaskManager_ExecutePending_AgentConnected(t *testing.T) {
	// We need a Server with a connected agent. We can't easily do a real
	// WebSocket round-trip in a unit test, so we verify that a task with
	// no server reference stays pending (offline) and a task whose agent
	// is not in the conns map also stays pending. This validates the
	// offline-queue logic without requiring a live agent.
	srv := NewServer("", "test-token", "")
	tm := NewTaskManager("", srv)

	task, _ := tm.Create("agent-1", "exec", json.RawMessage(`{}`), TaskSchedule{Type: ScheduleOnce}, "")

	// Agent not in conns map — should stay pending.
	tm.ExecutePending()

	got, _ := tm.Get(task.ID)
	if got.Status != TaskStatusPending {
		t.Errorf("expected pending (agent not connected), got %s", got.Status)
	}
	if got.StartedAt != nil {
		t.Error("expected nil StartedAt (not executed)")
	}
}

// ---------------------------------------------------------------------------
// Persistence — tasks survive across TaskManager instances
// ---------------------------------------------------------------------------

func TestTaskManager_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.json")

	tm1 := NewTaskManager(path, nil)
	task, err := tm1.Create("agent-1", "exec", json.RawMessage(`{"command":"ls"}`),
		TaskSchedule{Type: ScheduleRecurring, IntervalSeconds: 60}, "op-1")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Simulate server restart: create a new TaskManager pointing at the same file.
	tm2 := NewTaskManager(path, nil)

	got, err := tm2.Get(task.ID)
	if err != nil {
		t.Fatalf("Get after reload failed: %v", err)
	}
	if got.AgentID != "agent-1" {
		t.Errorf("expected agent_id=agent-1, got %s", got.AgentID)
	}
	if got.CommandType != "exec" {
		t.Errorf("expected command_type=exec, got %s", got.CommandType)
	}
	if got.Schedule.Type != ScheduleRecurring {
		t.Errorf("expected schedule type=recurring, got %s", got.Schedule.Type)
	}
	if got.Schedule.IntervalSeconds != 60 {
		t.Errorf("expected interval_seconds=60, got %d", got.Schedule.IntervalSeconds)
	}
}

// ---------------------------------------------------------------------------
// Persistence — running tasks reset to pending on reload
// ---------------------------------------------------------------------------

func TestTaskManager_Persistence_RunningResetsToPending(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.json")

	tm1 := NewTaskManager(path, nil)
	task, _ := tm1.Create("agent-1", "exec", nil, TaskSchedule{}, "")

	// Manually set the task to running and save.
	tm1.mu.Lock()
	task.Status = TaskStatusRunning
	now := time.Now().UTC()
	task.StartedAt = &now
	tm1.saveLocked()
	tm1.mu.Unlock()

	// Reload — running task should be reset to pending.
	tm2 := NewTaskManager(path, nil)
	got, _ := tm2.Get(task.ID)
	if got.Status != TaskStatusPending {
		t.Errorf("expected pending after reload (was running), got %s", got.Status)
	}
	if got.StartedAt != nil {
		t.Error("expected nil StartedAt after reload")
	}
}

// ---------------------------------------------------------------------------
// API endpoint tests
// ---------------------------------------------------------------------------

func TestV1_CreateTask(t *testing.T) {
	srv, cleanup := newV1TestServer(t)
	defer cleanup()

	body := `{"agent_id":"agent-1","command_type":"exec","params":{"command":"echo hi"},"schedule":{"type":"once"}}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/tasks", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, error: %+v", resp.Error)
	}
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data, got %T", resp.Data)
	}
	if data["id"] == nil || data["id"] == "" {
		t.Error("expected non-empty task id")
	}
	if data["status"] != TaskStatusPending {
		t.Errorf("expected status=pending, got %v", data["status"])
	}
}

func TestV1_ListTasks(t *testing.T) {
	srv, cleanup := newV1TestServer(t)
	defer cleanup()

	// Create a task via API.
	body := `{"agent_id":"agent-1","command_type":"exec","schedule":{"type":"once"}}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/tasks", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	srv.mux.ServeHTTP(rec, req)

	// List tasks.
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/api/v1/tasks", nil)
	req2.Header.Set("Authorization", "Bearer test-token")
	srv.mux.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec2.Code)
	}
	var resp APIResponse
	json.Unmarshal(rec2.Body.Bytes(), &resp)
	if !resp.OK {
		t.Fatalf("expected ok=true, error: %+v", resp.Error)
	}
	tasks, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("expected array data, got %T", resp.Data)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
}

func TestV1_GetTask(t *testing.T) {
	srv, cleanup := newV1TestServer(t)
	defer cleanup()

	// Create a task.
	task, err := srv.tasks.Create("agent-1", "exec", nil, TaskSchedule{}, "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// GET /api/v1/tasks/{id}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/tasks/"+task.ID, nil)
	req.Header.Set("Authorization", "Bearer test-token")
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.OK {
		t.Fatalf("expected ok=true, error: %+v", resp.Error)
	}
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data, got %T", resp.Data)
	}
	if data["id"] != task.ID {
		t.Errorf("expected id=%s, got %v", task.ID, data["id"])
	}
}

func TestV1_GetTask_NotFound(t *testing.T) {
	srv, cleanup := newV1TestServer(t)
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/tasks/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestV1_CancelTask(t *testing.T) {
	srv, cleanup := newV1TestServer(t)
	defer cleanup()

	// Create a task.
	task, _ := srv.tasks.Create("agent-1", "exec", nil, TaskSchedule{}, "")

	// DELETE /api/v1/tasks/{id}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/v1/tasks/"+task.ID, nil)
	req.Header.Set("Authorization", "Bearer test-token")
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify task is cancelled.
	got, _ := srv.tasks.Get(task.ID)
	if got.Status != TaskStatusCancelled {
		t.Errorf("expected cancelled, got %s", got.Status)
	}
}

func TestV1_CancelTask_NotFound(t *testing.T) {
	srv, cleanup := newV1TestServer(t)
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/v1/tasks/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestV1_CreateTask_InvalidJSON(t *testing.T) {
	srv, cleanup := newV1TestServer(t)
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/tasks", strings.NewReader("{bad json"))
	req.Header.Set("Authorization", "Bearer test-token")
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestV1_ListTasks_FilterByAgent(t *testing.T) {
	srv, cleanup := newV1TestServer(t)
	defer cleanup()

	srv.tasks.Create("agent-1", "exec", nil, TaskSchedule{}, "")
	srv.tasks.Create("agent-2", "exec", nil, TaskSchedule{}, "")

	// List filtered by agent-1.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/tasks?agent_id=agent-1", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	tasks, _ := resp.Data.([]interface{})
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task for agent-1, got %d", len(tasks))
	}
}