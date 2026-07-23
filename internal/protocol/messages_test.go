package protocol

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestEnvelope_MarshalUnmarshal_RoundTrip verifies that an Envelope
// survives a marshal → unmarshal cycle with all fields intact.
func TestEnvelope_MarshalUnmarshal_RoundTrip(t *testing.T) {
	params := json.RawMessage(`{"command":"echo hi"}`)
	result := json.RawMessage(`{"stdout":"hello\n"}`)
	errInfo := &ErrorInfo{Code: ErrPermissionDenied, Message: "denied"}

	original := Envelope{
		ID:     "test-001",
		Type:   TypeExecResult,
		Params: params,
		Result: result,
		Error:  errInfo,
		Bypass: true,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Envelope
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID: got %q, want %q", decoded.ID, original.ID)
	}
	if decoded.Type != original.Type {
		t.Errorf("Type: got %q, want %q", decoded.Type, original.Type)
	}
	if decoded.Bypass != original.Bypass {
		t.Errorf("Bypass: got %v, want %v", decoded.Bypass, original.Bypass)
	}
	if string(decoded.Params) != string(original.Params) {
		t.Errorf("Params: got %q, want %q", decoded.Params, original.Params)
	}
	if string(decoded.Result) != string(original.Result) {
		t.Errorf("Result: got %q, want %q", decoded.Result, original.Result)
	}
	if decoded.Error == nil {
		t.Fatal("Error: got nil")
	}
	if decoded.Error.Code != errInfo.Code {
		t.Errorf("Error.Code: got %q, want %q", decoded.Error.Code, errInfo.Code)
	}
	if decoded.Error.Message != errInfo.Message {
		t.Errorf("Error.Message: got %q, want %q", decoded.Error.Message, errInfo.Message)
	}
}

// TestEnvelope_OmitEmpty verifies that omitempty fields are not present
// in the JSON when zero.
func TestEnvelope_OmitEmpty(t *testing.T) {
	env := Envelope{ID: "test-002", Type: TypePing}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	if strings.Contains(s, "params") {
		t.Errorf("expected params to be omitted, got: %s", s)
	}
	if strings.Contains(s, "result") {
		t.Errorf("expected result to be omitted, got: %s", s)
	}
	if strings.Contains(s, "error") {
		t.Errorf("expected error to be omitted, got: %s", s)
	}
	if strings.Contains(s, "bypass") {
		t.Errorf("expected bypass to be omitted, got: %s", s)
	}
}

// TestNewError verifies NewError constructs a proper error envelope.
func TestNewError(t *testing.T) {
	env := NewError("req-123", ErrPermissionDenied, "access blocked")
	if env.ID != "req-123" {
		t.Errorf("ID: got %q, want %q", env.ID, "req-123")
	}
	if env.Type != TypeError {
		t.Errorf("Type: got %q, want %q", env.Type, TypeError)
	}
	if env.Error == nil {
		t.Fatal("Error is nil")
	}
	if env.Error.Code != ErrPermissionDenied {
		t.Errorf("Error.Code: got %q, want %q", env.Error.Code, ErrPermissionDenied)
	}
	if env.Error.Message != "access blocked" {
		t.Errorf("Error.Message: got %q, want %q", env.Error.Message, "access blocked")
	}
	if env.Result != nil {
		t.Errorf("Result should be nil for error, got %q", env.Result)
	}
}

// TestNewPing verifies NewPing creates a ping with an ID.
func TestNewPing(t *testing.T) {
	env := NewPing()
	if env.Type != TypePing {
		t.Errorf("Type: got %q, want %q", env.Type, TypePing)
	}
	if !strings.HasPrefix(env.ID, "ping-") {
		t.Errorf("ID should start with 'ping-', got %q", env.ID)
	}
}

// TestNewPong verifies NewPong echoes the ping ID.
func TestNewPong(t *testing.T) {
	ping := NewPing()
	pong := NewPong(ping.ID)
	if pong.ID != ping.ID {
		t.Errorf("Pong ID: got %q, want %q", pong.ID, ping.ID)
	}
	if pong.Type != TypePong {
		t.Errorf("Pong Type: got %q, want %q", pong.Type, TypePong)
	}
}

// TestNewResult verifies NewResult marshals the result correctly.
func TestNewResult(t *testing.T) {
	execResult := ExecResult{Stdout: "hello", ExitCode: 0}
	env := NewResult("req-456", TypeExecResult, execResult)
	if env.ID != "req-456" {
		t.Errorf("ID: got %q, want %q", env.ID, "req-456")
	}
	if env.Type != TypeExecResult {
		t.Errorf("Type: got %q, want %q", env.Type, TypeExecResult)
	}
	if env.Result == nil {
		t.Fatal("Result is nil")
	}
	var decoded ExecResult
	if err := json.Unmarshal(env.Result, &decoded); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if decoded.Stdout != "hello" {
		t.Errorf("Stdout: got %q, want %q", decoded.Stdout, "hello")
	}
	if decoded.ExitCode != 0 {
		t.Errorf("ExitCode: got %d, want %d", decoded.ExitCode, 0)
	}
}

// TestNewResult_MarshalError verifies that NewResult falls back to an
// error envelope when the result cannot be marshalled.
func TestNewResult_MarshalError(t *testing.T) {
	// Create a value that will fail to marshal (e.g. a channel)
	bad := make(chan int)
	env := NewResult("req-bad", TypeExecResult, bad)
	if env.Type != TypeError {
		t.Errorf("Type: got %q, want %q", env.Type, TypeError)
	}
	if env.Error == nil {
		t.Fatal("Error is nil")
	}
	if env.Error.Code != ErrInternal {
		t.Errorf("Error.Code: got %q, want %q", env.Error.Code, ErrInternal)
	}
}

// TestParseCommand verifies ParseCommand extracts typed params from an envelope.
func TestParseCommand(t *testing.T) {
	execParams := ExecParams{Command: "echo hi", Timeout: 30}
	paramsData, _ := json.Marshal(execParams)

	env := Envelope{
		ID:     "test-parse",
		Type:   TypeExec,
		Params: paramsData,
	}

	parsed, err := ParseCommand[ExecParams](env)
	if err != nil {
		t.Fatalf("ParseCommand: %v", err)
	}
	if parsed.Command != "echo hi" {
		t.Errorf("Command: got %q, want %q", parsed.Command, "echo hi")
	}
	if parsed.Timeout != 30 {
		t.Errorf("Timeout: got %d, want %d", parsed.Timeout, 30)
	}
}

// TestParseCommand_InvalidJSON verifies ParseCommand returns an error
// for malformed params.
func TestParseCommand_InvalidJSON(t *testing.T) {
	env := Envelope{
		ID:     "test-bad",
		Type:   TypeExec,
		Params: json.RawMessage(`{invalid json}`),
	}
	_, err := ParseCommand[ExecParams](env)
	if err == nil {
		t.Error("expected error for invalid JSON params")
	}
}

// TestAgentInfo_Serialization verifies AgentInfo round-trips through JSON.
func TestAgentInfo_Serialization(t *testing.T) {
	original := AgentInfo{
		Name:            "test-agent-001",
		Version:         "0.2.2",
		OS:              "linux",
		Arch:            "amd64",
		Mode:            "outbound",
		ProtocolVersion: "2",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded AgentInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("Name: got %q, want %q", decoded.Name, original.Name)
	}
	if decoded.Version != original.Version {
		t.Errorf("Version: got %q, want %q", decoded.Version, original.Version)
	}
	if decoded.OS != original.OS {
		t.Errorf("OS: got %q, want %q", decoded.OS, original.OS)
	}
	if decoded.Arch != original.Arch {
		t.Errorf("Arch: got %q, want %q", decoded.Arch, original.Arch)
	}
	if decoded.Mode != original.Mode {
		t.Errorf("Mode: got %q, want %q", decoded.Mode, original.Mode)
	}
	if decoded.ProtocolVersion != original.ProtocolVersion {
		t.Errorf("ProtocolVersion: got %q, want %q", decoded.ProtocolVersion, original.ProtocolVersion)
	}
}

// TestAgentInfo_OmitProtocolVersion verifies that ProtocolVersion is
// omitted when empty (old agent compat).
func TestAgentInfo_OmitProtocolVersion(t *testing.T) {
	info := AgentInfo{
		Name:    "old-agent",
		Version: "0.1.0",
		OS:      "windows",
		Arch:    "amd64",
		Mode:    "inbound",
		// ProtocolVersion intentionally empty
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	if strings.Contains(s, "protocol_version") {
		t.Errorf("expected protocol_version to be omitted, got: %s", s)
	}
}

// TestErrorCodes verifies that error code constants are defined and stable.
func TestErrorCodes(t *testing.T) {
	codes := map[string]string{
		"ErrTimeout":              ErrTimeout,
		"ErrPermissionDenied":     ErrPermissionDenied,
		"ErrNotFound":             ErrNotFound,
		"ErrInvalidParams":        ErrInvalidParams,
		"ErrPlatformNotSupported": ErrPlatformNotSupported,
		"ErrInternal":             ErrInternal,
	}

	expected := map[string]string{
		"ErrTimeout":              "TIMEOUT",
		"ErrPermissionDenied":     "PERMISSION_DENIED",
		"ErrNotFound":             "NOT_FOUND",
		"ErrInvalidParams":        "INVALID_PARAMS",
		"ErrPlatformNotSupported": "PLATFORM_NOT_SUPPORTED",
		"ErrInternal":             "INTERNAL_ERROR",
	}

	for name, val := range codes {
		if val != expected[name] {
			t.Errorf("%s: got %q, want %q", name, val, expected[name])
		}
	}
}

// TestAllMessageTypes_MarshalUnmarshal verifies that all message type
// constants survive a round-trip through an Envelope.
func TestAllMessageTypes_MarshalUnmarshal(t *testing.T) {
	types := []string{
		TypePing, TypePong,
		TypeExec, TypeExecPTY, TypeExecResult,
		TypeFSList, TypeFSStat, TypeFSRead, TypeFSHash,
		TypeFileSave, TypeFileRemove, TypeFSMove, TypeFSMkdir,
		TypeFSListResult, TypeFSStatResult, TypeFSReadResult,
		TypeFileSaveResult, TypeFileRemoveResult,
		TypeFSMoveResult, TypeFSMkdirResult, TypeFSHashResult,
		TypeCapture, TypeCaptureResult,
		TypeDisplayInfo, TypeDisplayInfoResult,
		TypePointerClick, TypeTextInput, TypeKeyPress, TypeKeyCombo,
		TypePointerClickResult, TypeTextInputResult,
		TypeKeyPressResult, TypeKeyComboResult,
		TypeHealth, TypeHealthResult,
		TypeTaskList, TypeTaskStop, TypeTaskListResult, TypeTaskStopResult,
		TypeOpenLink, TypeOpenLinkResult,
		TypeNotify, TypeNotifyResult,
		TypeClipboardRead, TypeClipboardWrite,
		TypeClipboardReadResult, TypeClipboardWriteResult,
		TypeTokenRotate, TypeTokenRefresh,
		TypeTokenRotateResult,
		TypeAuthRefresh, TypeAuthRequest,
		TypeAuthRefreshResult,
		TypeTunnelOpen, TypeTunnelClose, TypeTunnelData,
		TypeTunnelOpened, TypeTunnelClosed, TypeTunnelError,
		TypeSniffStart, TypeSniffStop,
		TypeMitmStart, TypeMitmStop, TypeMitmData,
		TypeMitmStarted, TypeMitmStopped,
		TypeDebugAttach, TypeDebugDetach,
		TypeDebugReadMem, TypeDebugModules, TypeDebugMemQuery,
		TypeProcList, TypeProcKill, TypeProcStart,
		TypeProcListResult, TypeProcKillResult, TypeProcStartResult,
		TypeAgentUpdate, TypeAgentUpdateResult,
		TypeStreamBegin, TypeStreamEnd,
		TypeStreamBeginResult, TypeStreamEndResult,
		// Phase 7 new types
		TypeSocks5Start, TypeSocks5Stop, TypeSocks5Result,
		TypePortForward, TypePortForwardResult,
		TypePortScan, TypePortScanResult,
		TypeNetConnections, TypeNetConnectionsResult,
		TypeAutostartEnable, TypeAutostartDisable, TypeAutostartStatus, TypeAutostartResult,
		TypeFileSearch, TypeFileSearchResult,
		TypeSysInfo, TypeSysInfoResult,
		TypeError,
	}

	for _, msgType := range types {
		env := Envelope{ID: "rt-" + msgType, Type: msgType}
		data, err := json.Marshal(env)
		if err != nil {
			t.Fatalf("marshal %s: %v", msgType, err)
		}
		var decoded Envelope
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal %s: %v", msgType, err)
		}
		if decoded.Type != msgType {
			t.Errorf("Type round-trip: got %q, want %q", decoded.Type, msgType)
		}
		if decoded.ID != "rt-"+msgType {
			t.Errorf("ID round-trip for %s: got %q, want %q", msgType, decoded.ID, "rt-"+msgType)
		}
	}
}

// TestTokenRotateParams_Serialization verifies TokenRotateParams with
// expiry round-trips correctly.
func TestTokenRotateParams_Serialization(t *testing.T) {
	now := time.Now().Truncate(time.Second) // truncate for JSON precision
	original := TokenRotateParams{
		NewToken: "new-secret-token",
		Expiry:   now,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded TokenRotateParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.NewToken != original.NewToken {
		t.Errorf("NewToken: got %q, want %q", decoded.NewToken, original.NewToken)
	}
	// time.Time JSON precision may differ by sub-second; compare within 1s
	if decoded.Expiry.Sub(original.Expiry).Abs() > time.Second {
		t.Errorf("Expiry: got %v, want %v", decoded.Expiry, original.Expiry)
	}
}

// TestExecParams_Serialization verifies ExecParams round-trips.
func TestExecParams_Serialization(t *testing.T) {
	original := ExecParams{
		Command: "ls -la /tmp",
		Timeout: 120,
		WorkDir: "/tmp",
		Env:     map[string]string{"PATH": "/usr/bin", "HOME": "/root"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ExecParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Command != original.Command {
		t.Errorf("Command: got %q, want %q", decoded.Command, original.Command)
	}
	if decoded.Timeout != original.Timeout {
		t.Errorf("Timeout: got %d, want %d", decoded.Timeout, original.Timeout)
	}
	if decoded.WorkDir != original.WorkDir {
		t.Errorf("WorkDir: got %q, want %q", decoded.WorkDir, original.WorkDir)
	}
	if decoded.Env["PATH"] != original.Env["PATH"] {
		t.Errorf("Env[PATH]: got %q, want %q", decoded.Env["PATH"], original.Env["PATH"])
	}
}

// TestFSParams_Serialization verifies FSParams round-trips.
func TestFSParams_Serialization(t *testing.T) {
	original := FSParams{
		Path:   "/tmp/sandbox/file.txt",
		Offset: 1024,
		Limit:  512,
		Data:   "aGVsbG8=", // base64 "hello"
		Mode:   "0644",
		From:   "/tmp/old",
		To:     "/tmp/new",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded FSParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Path != original.Path {
		t.Errorf("Path: got %q, want %q", decoded.Path, original.Path)
	}
	if decoded.Offset != original.Offset {
		t.Errorf("Offset: got %d, want %d", decoded.Offset, original.Offset)
	}
	if decoded.Limit != original.Limit {
		t.Errorf("Limit: got %d, want %d", decoded.Limit, original.Limit)
	}
	if decoded.Data != original.Data {
		t.Errorf("Data: got %q, want %q", decoded.Data, original.Data)
	}
	if decoded.Mode != original.Mode {
		t.Errorf("Mode: got %q, want %q", decoded.Mode, original.Mode)
	}
	if decoded.From != original.From {
		t.Errorf("From: got %q, want %q", decoded.From, original.From)
	}
	if decoded.To != original.To {
		t.Errorf("To: got %q, want %q", decoded.To, original.To)
	}
}

// TestErrorInfo_Serialization verifies ErrorInfo round-trips.
func TestErrorInfo_Serialization(t *testing.T) {
	original := ErrorInfo{Code: ErrNotFound, Message: "agent not found"}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ErrorInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Code != original.Code {
		t.Errorf("Code: got %q, want %q", decoded.Code, original.Code)
	}
	if decoded.Message != original.Message {
		t.Errorf("Message: got %q, want %q", decoded.Message, original.Message)
	}
}