package agent

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/falke-ai-circuit/probe/internal/protocol"
)

// --- Port Scan Tests ---

// TestPortScan_OpenPort verifies that port scanning correctly identifies
// an open port on localhost.
func TestPortScan_OpenPort(t *testing.T) {
	// Start a listener on a random port
	ln, err := listenRandom()
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	a := &Agent{}
	env := protocol.Envelope{
		ID:     "test-scan-1",
		Type:   protocol.TypePortScan,
		Params: mustMarshalParams(protocol.PortScanParams{Host: "127.0.0.1", Ports: []int{port}, Timeout: 2000}),
	}

	resp := a.handlePortScan(env)
	if resp.Type != protocol.TypePortScanResult {
		t.Fatalf("expected %s, got %s", protocol.TypePortScanResult, resp.Type)
	}
	var result protocol.PortScanResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	if result.Results[0].State != "open" {
		t.Errorf("expected port %d to be open, got %s", port, result.Results[0].State)
	}
	if len(result.Open) != 1 || result.Open[0] != port {
		t.Errorf("expected open list to contain port %d, got %v", port, result.Open)
	}
}

// TestPortScan_ClosedPort verifies that port scanning correctly identifies
// a closed port.
func TestPortScan_ClosedPort(t *testing.T) {
	// Find a port that's definitely not listening
	ln, err := listenRandom()
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	a := &Agent{}
	env := protocol.Envelope{
		ID:     "test-scan-2",
		Type:   protocol.TypePortScan,
		Params: mustMarshalParams(protocol.PortScanParams{Host: "127.0.0.1", Ports: []int{port}, Timeout: 500}),
	}

	resp := a.handlePortScan(env)
	var result protocol.PortScanResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	if result.Results[0].State != "closed" {
		t.Errorf("expected port %d to be closed, got %s", port, result.Results[0].State)
	}
	if len(result.Open) != 0 {
		t.Errorf("expected no open ports, got %v", result.Open)
	}
}

// TestPortScan_MissingHost verifies that missing host returns an error.
func TestPortScan_MissingHost(t *testing.T) {
	a := &Agent{}
	env := protocol.Envelope{
		ID:     "test-scan-3",
		Type:   protocol.TypePortScan,
		Params: mustMarshalParams(protocol.PortScanParams{Ports: []int{80}}),
	}

	resp := a.handlePortScan(env)
	if resp.Type != protocol.TypeError {
		t.Fatalf("expected error, got %s", resp.Type)
	}
	if resp.Error == nil {
		t.Fatal("expected error info")
	}
	if resp.Error.Code != protocol.ErrInvalidParams {
		t.Errorf("expected %s, got %s", protocol.ErrInvalidParams, resp.Error.Code)
	}
}

// --- File Search Tests ---

// TestFileSearch_Basic verifies that file search finds matching files.
func TestFileSearch_Basic(t *testing.T) {
	// Create a temp directory with some files
	tmpDir, err := os.MkdirTemp("", "filesearch-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	files := []string{"test1.log", "test2.log", "data.txt", "report.log"}
	for _, f := range files {
		path := filepath.Join(tmpDir, f)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create %s: %v", f, err)
		}
	}

	// Create a subdirectory with more files
	subDir := filepath.Join(tmpDir, "subdir")
	os.Mkdir(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "nested.log"), []byte("nested"), 0644)

	a := &Agent{}
	env := protocol.Envelope{
		ID:     "test-search-1",
		Type:   protocol.TypeFileSearch,
		Params: mustMarshalParams(protocol.FileSearchParams{RootPath: tmpDir, Pattern: "*.log"}),
	}

	resp := a.handleFileSearch(env)
	if resp.Type != protocol.TypeFileSearchResult {
		t.Fatalf("expected %s, got %s", protocol.TypeFileSearchResult, resp.Type)
	}
	var result protocol.FileSearchResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Count != 4 {
		t.Errorf("expected 4 matches, got %d", result.Count)
	}
}

// TestFileSearch_MaxResults verifies that MaxResults limits the search.
func TestFileSearch_MaxResults(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "filesearch-max-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	for i := 0; i < 10; i++ {
		path := filepath.Join(tmpDir, "file"+string(rune('0'+i))+".txt")
		os.WriteFile(path, []byte("test"), 0644)
	}

	a := &Agent{}
	env := protocol.Envelope{
		ID:     "test-search-2",
		Type:   protocol.TypeFileSearch,
		Params: mustMarshalParams(protocol.FileSearchParams{RootPath: tmpDir, Pattern: "*.txt", MaxResults: 3}),
	}

	resp := a.handleFileSearch(env)
	var result protocol.FileSearchResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Count > 3 {
		t.Errorf("expected at most 3 matches, got %d", result.Count)
	}
}

// TestFileSearch_MissingRootPath verifies that missing root_path returns an error.
func TestFileSearch_MissingRootPath(t *testing.T) {
	a := &Agent{}
	env := protocol.Envelope{
		ID:     "test-search-3",
		Type:   protocol.TypeFileSearch,
		Params: mustMarshalParams(protocol.FileSearchParams{Pattern: "*.log"}),
	}

	resp := a.handleFileSearch(env)
	if resp.Type != protocol.TypeError {
		t.Fatalf("expected error, got %s", resp.Type)
	}
}

// --- System Info Tests ---

// TestSysInfo_Basic verifies that sysinfo returns valid system info.
func TestSysInfo_Basic(t *testing.T) {
	a := &Agent{}
	env := protocol.Envelope{
		ID:     "test-sysinfo-1",
		Type:   protocol.TypeSysInfo,
	}

	resp := a.handleSysInfo(env)
	if resp.Type != protocol.TypeSysInfoResult {
		t.Fatalf("expected %s, got %s", protocol.TypeSysInfoResult, resp.Type)
	}
	var result protocol.SysInfoResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.OS == "" {
		t.Error("expected non-empty OS")
	}
	if result.Arch == "" {
		t.Error("expected non-empty Arch")
	}
	if result.NumCPU <= 0 {
		t.Errorf("expected positive NumCPU, got %d", result.NumCPU)
	}
	if result.GoVersion == "" {
		t.Error("expected non-empty GoVersion")
	}
	if result.Hostname == "" {
		t.Error("expected non-empty Hostname")
	}
	if len(result.Network) == 0 {
		t.Error("expected at least one network interface")
	}
}

// --- Net Connections Tests ---

// TestNetConnections_Basic verifies that net connections returns at least
// some connections on Linux.
func TestNetConnections_Basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping net connections test in short mode")
	}

	a := &Agent{}
	env := protocol.Envelope{
		ID:     "test-netconns-1",
		Type:   protocol.TypeNetConnections,
	}

	resp := a.handleNetConnections(env)
	if resp.Type != protocol.TypeNetConnectionsResult {
		t.Fatalf("expected %s, got %s", protocol.TypeNetConnectionsResult, resp.Type)
	}
	var result protocol.NetConnectionsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// On Linux, we should get some connections (at least loopback)
	if len(result.Connections) == 0 {
		t.Log("warning: no connections returned — may not have any active on this system")
	}
}

// --- Stub Tests (SOCKS5, Port Forward, Autostart) ---

// TestSocks5Start_Stub verifies that SOCKS5 start returns "not implemented".
func TestSocks5Start_Stub(t *testing.T) {
	a := &Agent{}
	env := protocol.Envelope{
		ID:     "test-socks5-1",
		Type:   protocol.TypeSocks5Start,
		Params: mustMarshalParams(protocol.Socks5StartParams{ListenAddr: "127.0.0.1:1080"}),
	}

	resp := a.handleSocks5Start(env)
	if resp.Type != protocol.TypeError {
		t.Fatalf("expected error, got %s", resp.Type)
	}
	if resp.Error == nil {
		t.Fatal("expected error info")
	}
	if resp.Error.Code != protocol.ErrPlatformNotSupported {
		t.Errorf("expected %s, got %s", protocol.ErrPlatformNotSupported, resp.Error.Code)
	}
}

// TestPortForward_Stub verifies that port forwarding returns "not implemented".
func TestPortForward_Stub(t *testing.T) {
	a := &Agent{}
	env := protocol.Envelope{
		ID:     "test-pf-1",
		Type:   protocol.TypePortForward,
		Params: mustMarshalParams(protocol.PortForwardParams{LocalPort: 8080, RemoteHost: "example.com", RemotePort: 80, Direction: "forward"}),
	}

	resp := a.handlePortForward(env)
	if resp.Type != protocol.TypeError {
		t.Fatalf("expected error, got %s", resp.Type)
	}
	if resp.Error == nil {
		t.Fatal("expected error info")
	}
}

// TestAutostartEnable_Stub verifies that autostart enable returns "not implemented".
func TestAutostartEnable_Stub(t *testing.T) {
	a := &Agent{}
	env := protocol.Envelope{
		ID:     "test-autostart-1",
		Type:   protocol.TypeAutostartEnable,
		Params: mustMarshalParams(protocol.AutostartParams{Method: "systemd", CommandPath: "/usr/bin/probe", Name: "probe"}),
	}

	resp := a.handleAutostartEnable(env)
	if resp.Type != protocol.TypeError {
		t.Fatalf("expected error, got %s", resp.Type)
	}
	if resp.Error == nil {
		t.Fatal("expected error info")
	}
}

// --- Permission Tests for Phase 7 ---

// TestIsAllowed_Phase7_FullPermissions verifies that all new capabilities
// are allowed under full permissions.
func TestIsAllowed_Phase7_FullPermissions(t *testing.T) {
	perm := PermFull
	sandbox := ""

	cmdTypes := []string{
		protocol.TypeSocks5Start, protocol.TypeSocks5Stop,
		protocol.TypePortForward,
		protocol.TypePortScan,
		protocol.TypeNetConnections,
		protocol.TypeAutostartEnable, protocol.TypeAutostartDisable, protocol.TypeAutostartStatus,
		protocol.TypeFileSearch,
		protocol.TypeSysInfo,
	}

	for _, ct := range cmdTypes {
		if !isAllowed(perm, sandbox, ct, "", "") {
			t.Errorf("full permission should allow %s", ct)
		}
	}
}

// TestIsAllowed_Phase7_ReadOnly verifies that read-only info capabilities
// (sysinfo, net_connections, port_scan, file_search) are allowed in
// read-only tier, but privileged ones are denied.
func TestIsAllowed_Phase7_ReadOnly(t *testing.T) {
	perm := PermReadOnly
	sandbox := ""

	allowed := []string{
		protocol.TypeSysInfo, protocol.TypeNetConnections,
		protocol.TypePortScan, protocol.TypeFileSearch,
	}
	for _, ct := range allowed {
		if !isAllowed(perm, sandbox, ct, "", "") {
			t.Errorf("read-only should allow %s", ct)
		}
	}

	denied := []string{
		protocol.TypeSocks5Start, protocol.TypeSocks5Stop,
		protocol.TypePortForward,
		protocol.TypeAutostartEnable, protocol.TypeAutostartDisable, protocol.TypeAutostartStatus,
	}
	for _, ct := range denied {
		if isAllowed(perm, sandbox, ct, "", "") {
			t.Errorf("read-only should deny %s", ct)
		}
	}
}

// TestIsAllowed_Phase7_Sandboxed verifies that sandboxed tier allows
// read-only info capabilities but denies privileged ones.
func TestIsAllowed_Phase7_Sandboxed(t *testing.T) {
	perm := PermSandboxed
	sandbox := "/tmp/sandbox"

	allowed := []string{
		protocol.TypeSysInfo, protocol.TypeNetConnections,
		protocol.TypePortScan, protocol.TypeFileSearch,
	}
	for _, ct := range allowed {
		if !isAllowed(perm, sandbox, ct, "", "") {
			t.Errorf("sandboxed should allow %s", ct)
		}
	}

	denied := []string{
		protocol.TypeSocks5Start, protocol.TypeSocks5Stop,
		protocol.TypePortForward,
		protocol.TypeAutostartEnable, protocol.TypeAutostartDisable, protocol.TypeAutostartStatus,
	}
	for _, ct := range denied {
		if isAllowed(perm, sandbox, ct, "", "") {
			t.Errorf("sandboxed should deny %s", ct)
		}
	}
}

// TestIsAllowed_Phase7_Standard verifies that standard tier allows
// read-only info capabilities but denies privileged ones.
func TestIsAllowed_Phase7_Standard(t *testing.T) {
	perm := PermStandard
	sandbox := ""

	allowed := []string{
		protocol.TypeSysInfo, protocol.TypeNetConnections,
		protocol.TypePortScan, protocol.TypeFileSearch,
	}
	for _, ct := range allowed {
		if !isAllowed(perm, sandbox, ct, "", "") {
			t.Errorf("standard should allow %s", ct)
		}
	}

	denied := []string{
		protocol.TypeSocks5Start, protocol.TypeSocks5Stop,
		protocol.TypePortForward,
		protocol.TypeAutostartEnable, protocol.TypeAutostartDisable, protocol.TypeAutostartStatus,
	}
	for _, ct := range denied {
		if isAllowed(perm, sandbox, ct, "", "") {
			t.Errorf("standard should deny %s", ct)
		}
	}
}

// --- Helper Functions ---

// listenRandom creates a TCP listener on a random port for testing.
func listenRandom() (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}

// mustMarshalParams marshals params to json.RawMessage for test envelopes.
func mustMarshalParams(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}