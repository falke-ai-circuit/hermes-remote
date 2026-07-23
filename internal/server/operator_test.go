package server

import (
	"testing"
)

// TestOperator_CanPerform verifies role-based permission checks.
func TestOperator_CanPerform(t *testing.T) {
	tests := []struct {
		role    string
		action  string
		want    bool
	}{
		{RoleAdmin, "exec", true},
		{RoleAdmin, "fs-write", true},
		{RoleAdmin, "anything", true},
		{RoleOperator, "exec", true},
		{RoleOperator, "fs-write", true},
		{RoleOperator, "proc-kill", true},
		{RoleOperator, "anything", true},
		{RoleViewer, "list", true},
		{RoleViewer, "health", true},
		{RoleViewer, "fs-read", true},
		{RoleViewer, "fs-stat", true},
		{RoleViewer, "fs-list", true},
		{RoleViewer, "fs-hash", true},
		{RoleViewer, "exec", false},
		{RoleViewer, "fs-write", false},
		{RoleViewer, "proc-kill", false},
		{RoleViewer, "capture", false},
		{"unknown", "exec", false},
		{"", "list", false},
	}
	for _, tt := range tests {
		op := &Operator{Role: tt.role}
		if got := op.CanPerform(tt.action); got != tt.want {
			t.Errorf("role=%q action=%q: got %v, want %v", tt.role, tt.action, got, tt.want)
		}
	}
}

// TestOperatorManager_Create verifies that Create adds a new operator.
func TestOperatorManager_Create(t *testing.T) {
	om := NewOperatorManager("")
	op, err := om.Create("alice", RoleAdmin, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if op.ID == "" {
		t.Error("ID should not be empty")
	}
	if op.Name != "alice" {
		t.Errorf("Name: got %q, want %q", op.Name, "alice")
	}
	if op.Role != RoleAdmin {
		t.Errorf("Role: got %q, want %q", op.Role, RoleAdmin)
	}
	if op.Token == "" {
		t.Error("Token should not be empty")
	}
	if op.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

// TestOperatorManager_Create_InvalidRole verifies that an invalid role is rejected.
func TestOperatorManager_Create_InvalidRole(t *testing.T) {
	om := NewOperatorManager("")
	_, err := om.Create("bob", "superuser", "")
	if err == nil {
		t.Error("expected error for invalid role")
	}
}

// TestOperatorManager_GetByToken verifies token-based lookup.
func TestOperatorManager_GetByToken(t *testing.T) {
	om := NewOperatorManager("")
	op, _ := om.Create("alice", RoleOperator, "my-secret-token")

	found := om.GetByToken("my-secret-token")
	if found == nil {
		t.Fatal("expected operator for token")
	}
	if found.ID != op.ID {
		t.Errorf("ID: got %q, want %q", found.ID, op.ID)
	}
	if found.Name != "alice" {
		t.Errorf("Name: got %q, want %q", found.Name, "alice")
	}
}

// TestOperatorManager_GetByToken_NotFound verifies nil for unknown token.
func TestOperatorManager_GetByToken_NotFound(t *testing.T) {
	om := NewOperatorManager("")
	if om.GetByToken("nonexistent") != nil {
		t.Error("expected nil for unknown token")
	}
	if om.GetByToken("") != nil {
		t.Error("expected nil for empty token")
	}
}

// TestOperatorManager_Get verifies ID-based lookup.
func TestOperatorManager_Get(t *testing.T) {
	om := NewOperatorManager("")
	op, _ := om.Create("alice", RoleAdmin, "")

	found := om.Get(op.ID)
	if found == nil {
		t.Fatal("expected operator")
	}
	if found.ID != op.ID {
		t.Errorf("ID: got %q, want %q", found.ID, op.ID)
	}
}

// TestOperatorManager_Get_NotFound verifies nil for unknown ID.
func TestOperatorManager_Get_NotFound(t *testing.T) {
	om := NewOperatorManager("")
	if om.Get("no-such-id") != nil {
		t.Error("expected nil for unknown ID")
	}
}

// TestOperatorManager_List verifies multiple operators are returned.
func TestOperatorManager_List(t *testing.T) {
	om := NewOperatorManager("")
	om.Create("alice", RoleAdmin, "tok-1")
	om.Create("bob", RoleOperator, "tok-2")
	om.Create("carol", RoleViewer, "tok-3")

	ops := om.List()
	if len(ops) != 3 {
		t.Fatalf("expected 3 operators, got %d", len(ops))
	}
}

// TestOperatorManager_Delete verifies removal.
func TestOperatorManager_Delete(t *testing.T) {
	om := NewOperatorManager("")
	op, _ := om.Create("alice", RoleAdmin, "my-token")

	if !om.Delete(op.ID) {
		t.Error("expected Delete to return true for existing operator")
	}
	if om.Get(op.ID) != nil {
		t.Error("operator should not exist after Delete")
	}
	if om.GetByToken("my-token") != nil {
		t.Error("token should be invalidated after Delete")
	}
}

// TestOperatorManager_Delete_NonExistent verifies false for unknown ID.
func TestOperatorManager_Delete_NonExistent(t *testing.T) {
	om := NewOperatorManager("")
	if om.Delete("no-such-id") {
		t.Error("expected false for deleting non-existent operator")
	}
}

// TestOperatorManager_RotateToken verifies that rotation invalidates the old
// token and produces a new one.
func TestOperatorManager_RotateToken(t *testing.T) {
	om := NewOperatorManager("")
	op, _ := om.Create("alice", RoleAdmin, "old-token")

	newToken, err := om.RotateToken(op.ID)
	if err != nil {
		t.Fatalf("RotateToken: %v", err)
	}
	if newToken == "" {
		t.Error("new token should not be empty")
	}
	if newToken == "old-token" {
		t.Error("new token should differ from old token")
	}
	// Old token should be invalid.
	if om.GetByToken("old-token") != nil {
		t.Error("old token should be invalid after rotation")
	}
	// New token should work.
	if om.GetByToken(newToken) == nil {
		t.Error("new token should be valid after rotation")
	}
}

// TestOperatorManager_RotateToken_NotFound verifies error for unknown ID.
func TestOperatorManager_RotateToken_NotFound(t *testing.T) {
	om := NewOperatorManager("")
	_, err := om.RotateToken("no-such-id")
	if err == nil {
		t.Error("expected error for non-existent operator")
	}
}

// TestOperatorManager_IsEmpty verifies the empty check.
func TestOperatorManager_IsEmpty(t *testing.T) {
	om := NewOperatorManager("")
	if !om.IsEmpty() {
		t.Error("expected empty manager to be empty")
	}
	om.Create("alice", RoleAdmin, "")
	if om.IsEmpty() {
		t.Error("expected non-empty after Create")
	}
}

// TestOperatorManager_Persistence verifies that operators persist to disk.
func TestOperatorManager_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/operators.json"

	om1 := NewOperatorManager(path)
	op, _ := om1.Create("alice", RoleAdmin, "secret-token")
	om1.Create("bob", RoleViewer, "bob-token")

	// Reload from disk.
	om2 := NewOperatorManager(path)
	if om2.IsEmpty() {
		t.Fatal("expected operators after reload")
	}
	ops := om2.List()
	if len(ops) != 2 {
		t.Fatalf("expected 2 operators after reload, got %d", len(ops))
	}
	// Token should be persisted.
	if om2.GetByToken("secret-token") == nil {
		t.Error("token not persisted")
	}
	if om2.GetByToken("bob-token") == nil {
		t.Error("bob token not persisted")
	}
	// ID should match.
	if om2.Get(op.ID) == nil {
		t.Error("operator ID not persisted")
	}
}

// TestOperatorManager_UpdateLastSeen verifies the last-seen timestamp is set.
func TestOperatorManager_UpdateLastSeen(t *testing.T) {
	om := NewOperatorManager("")
	op, _ := om.Create("alice", RoleAdmin, "")

	om.UpdateLastSeen(op.ID, testTime)
	found := om.Get(op.ID)
	if found == nil {
		t.Fatal("expected operator")
	}
	if !found.LastSeen.Equal(testTime) {
		t.Errorf("LastSeen: got %v, want %v", found.LastSeen, testTime)
	}
}
// TestOperator_SetPassword_CheckPassword verifies bcrypt password set/check.
func TestOperator_SetPassword_CheckPassword(t *testing.T) {
	op := &Operator{}
	if err := op.SetPassword("hunter2"); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
	if op.PasswordHash == "" {
		t.Fatal("PasswordHash should not be empty after SetPassword")
	}
	if op.PasswordHash == "hunter2" {
		t.Fatal("PasswordHash should not be plaintext")
	}
	if !op.CheckPassword("hunter2") {
		t.Error("CheckPassword should return true for correct password")
	}
	if op.CheckPassword("wrong") {
		t.Error("CheckPassword should return false for wrong password")
	}
	// Empty hash → always false.
	op2 := &Operator{}
	if op2.CheckPassword("anything") {
		t.Error("CheckPassword should return false when no hash set")
	}
}

// TestOperatorManager_CreateWithPassword verifies operator creation with password.
func TestOperatorManager_CreateWithPassword(t *testing.T) {
	om := NewOperatorManager("")
	op, err := om.CreateWithPassword("alice", RoleAdmin, "secret123", "")
	if err != nil {
		t.Fatalf("CreateWithPassword: %v", err)
	}
	if op.PasswordHash == "" {
		t.Error("PasswordHash should be set")
	}
	if !op.CheckPassword("secret123") {
		t.Error("CheckPassword should match")
	}
}

// TestOperatorManager_GetByName verifies name-based lookup.
func TestOperatorManager_GetByName(t *testing.T) {
	om := NewOperatorManager("")
	om.Create("alice", RoleAdmin, "tok-1")
	found := om.GetByName("alice")
	if found == nil {
		t.Fatal("expected operator for name 'alice'")
	}
	if found.Name != "alice" {
		t.Errorf("Name: got %q, want %q", found.Name, "alice")
	}
	if om.GetByName("nonexistent") != nil {
		t.Error("expected nil for unknown name")
	}
	if om.GetByName("") != nil {
		t.Error("expected nil for empty name")
	}
}

// TestOperatorManager_PasswordPersistence verifies password hash persists to disk.
func TestOperatorManager_PasswordPersistence(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/operators.json"

	om1 := NewOperatorManager(path)
	op, _ := om1.CreateWithPassword("alice", RoleAdmin, "mypass", "tok-1")

	// Reload from disk.
	om2 := NewOperatorManager(path)
	found := om2.GetByName("alice")
	if found == nil {
		t.Fatal("expected operator after reload")
	}
	if !found.CheckPassword("mypass") {
		t.Error("password should persist across reload")
	}
	_ = op
}
