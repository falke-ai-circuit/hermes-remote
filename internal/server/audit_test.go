package server

import (
	"os"
	"testing"
	"time"
)

// testTime is a fixed timestamp used in tests.
var testTime = time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)

// TestAuditLogger_Log verifies that entries are written to the JSONL file.
func TestAuditLogger_Log(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/audit.jsonl"
	al := NewAuditLogger(path)

	al.Log(AuditEntry{
		AgentID:    "agent-1",
		OperatorID: "op-1",
		Action:     "exec",
		Result:     "success",
	})

	al.Log(AuditEntry{
		AgentID:    "agent-2",
		OperatorID: "op-2",
		Action:     "fs-read",
		Result:     "error",
		ErrorMsg:   "file not found",
	})

	// Verify file has 2 lines.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := 0
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}
	if lines != 2 {
		t.Errorf("expected 2 lines, got %d", lines)
	}
}

// TestAuditLogger_Query verifies filtering by agent_id.
func TestAuditLogger_Query(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/audit.jsonl"
	al := NewAuditLogger(path)

	al.Log(AuditEntry{AgentID: "a1", OperatorID: "op1", Action: "exec", Result: "success"})
	al.Log(AuditEntry{AgentID: "a2", OperatorID: "op2", Action: "fs-read", Result: "success"})
	al.Log(AuditEntry{AgentID: "a1", OperatorID: "op1", Action: "fs-write", Result: "success"})

	// Filter by agent_id.
	results := al.Query(AuditFilter{AgentID: "a1"})
	if len(results) != 2 {
		t.Fatalf("expected 2 entries for a1, got %d", len(results))
	}
	for _, e := range results {
		if e.AgentID != "a1" {
			t.Errorf("AgentID: got %q, want %q", e.AgentID, "a1")
		}
	}
}

// TestAuditLogger_Query_ByAction verifies filtering by action.
func TestAuditLogger_Query_ByAction(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/audit.jsonl"
	al := NewAuditLogger(path)

	al.Log(AuditEntry{AgentID: "a1", Action: "exec", Result: "success"})
	al.Log(AuditEntry{AgentID: "a2", Action: "fs-read", Result: "success"})
	al.Log(AuditEntry{AgentID: "a3", Action: "exec", Result: "success"})

	results := al.Query(AuditFilter{Action: "exec"})
	if len(results) != 2 {
		t.Fatalf("expected 2 entries for exec, got %d", len(results))
	}
}

// TestAuditLogger_Query_ByOperator verifies filtering by operator_id.
func TestAuditLogger_Query_ByOperator(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/audit.jsonl"
	al := NewAuditLogger(path)

	al.Log(AuditEntry{AgentID: "a1", OperatorID: "op1", Action: "exec", Result: "success"})
	al.Log(AuditEntry{AgentID: "a2", OperatorID: "op2", Action: "exec", Result: "success"})

	results := al.Query(AuditFilter{OperatorID: "op1"})
	if len(results) != 1 {
		t.Fatalf("expected 1 entry for op1, got %d", len(results))
	}
	if results[0].OperatorID != "op1" {
		t.Errorf("OperatorID: got %q, want %q", results[0].OperatorID, "op1")
	}
}

// TestAuditLogger_Query_ByTimeRange verifies time-based filtering.
func TestAuditLogger_Query_ByTimeRange(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/audit.jsonl"
	al := NewAuditLogger(path)

	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

	al.Log(AuditEntry{AgentID: "a1", Action: "exec", Timestamp: t1, Result: "success"})
	al.Log(AuditEntry{AgentID: "a2", Action: "exec", Timestamp: t2, Result: "success"})
	al.Log(AuditEntry{AgentID: "a3", Action: "exec", Timestamp: t3, Result: "success"})

	// Filter: from t2 to t3 (exclusive-ish).
	from := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	results := al.Query(AuditFilter{FromTime: from, ToTime: to})
	if len(results) != 1 {
		t.Fatalf("expected 1 entry in time range, got %d", len(results))
	}
	if results[0].AgentID != "a2" {
		t.Errorf("AgentID: got %q, want %q", results[0].AgentID, "a2")
	}
}

// TestAuditLogger_Query_Limit verifies the limit returns the most recent N.
func TestAuditLogger_Query_Limit(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/audit.jsonl"
	al := NewAuditLogger(path)

	for i := 0; i < 10; i++ {
		al.Log(AuditEntry{
			AgentID:   "a1",
			Action:    "exec",
			Timestamp: time.Date(2026, 1, 1, 0, i, 0, 0, time.UTC),
			Result:    "success",
		})
	}

	results := al.Query(AuditFilter{Limit: 3})
	if len(results) != 3 {
		t.Fatalf("expected 3 entries with limit, got %d", len(results))
	}
	// Should be the last 3 (most recent).
	if results[2].Timestamp.Minute() != 9 {
		t.Errorf("expected last entry at minute 9, got %d", results[2].Timestamp.Minute())
	}
}

// TestAuditLogger_EmptyPath verifies that logging with no path is a no-op.
func TestAuditLogger_EmptyPath(t *testing.T) {
	al := NewAuditLogger("")
	al.Log(AuditEntry{AgentID: "a1", Action: "exec", Result: "success"})
	// Should not panic, should return nil from Query.
	if results := al.Query(AuditFilter{}); results != nil {
		t.Errorf("expected nil results for empty path, got %d", len(results))
	}
}

// TestAuditLogger_Query_NoFile verifies Query on a non-existent file returns nil.
func TestAuditLogger_Query_NoFile(t *testing.T) {
	al := NewAuditLogger("/nonexistent/path/audit.jsonl")
	if results := al.Query(AuditFilter{}); results != nil {
		t.Errorf("expected nil for non-existent file, got %d", len(results))
	}
}

// TestAuditLogger_Log_SetsTimestamp verifies that a zero timestamp is filled in.
func TestAuditLogger_Log_SetsTimestamp(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/audit.jsonl"
	al := NewAuditLogger(path)

	al.Log(AuditEntry{AgentID: "a1", Action: "exec", Result: "success"})

	results := al.Query(AuditFilter{})
	if len(results) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(results))
	}
	if results[0].Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}