package protocol

import (
	"encoding/json"
	"fmt"
	"time"
)

// Message types
const (
	// Agent → Server
	TypePing  = "ping"
	TypePong  = "pong"

	// Commands (Server → Agent)
	TypeExec          = "exec"
	TypeExecPTY       = "exec_pty"
	TypeFSList        = "fs_list"
	TypeFSStat        = "fs_stat"
	TypeFSRead        = "fs_read"
	TypeFileSave      = "file_save"
	TypeFileRemove    = "file_remove"
	TypeFSMove        = "fs_move"
	TypeFSMkdir       = "fs_mkdir"
	TypeFSHash        = "fs_hash"
	TypeCapture       = "capture"
	TypeDisplayRegion = "display_region"
	TypeDisplayInfo   = "display_info"
	TypePointerClick  = "pointer_click"
	TypeTextInput     = "text_input"
	TypeKeyPress      = "keypress"
	TypeKeyCombo      = "keycombo"
	TypeHealth        = "health"
	TypeTaskList      = "task_list"
	TypeTaskStop      = "task_stop"
	TypeOpenLink      = "open_link"
	TypeNotify        = "notify"
	TypeClipboardRead  = "clipboard_read"
	TypeClipboardWrite = "clipboard_write"
	TypeTokenRotate    = "token_rotate"    // Server → Agent: rotate token (new name)
	TypeAuthRefresh    = "auth_refresh"    // DEPRECATED alias for TypeTokenRotate (backward compat)
	TypeTokenRefresh   = "token_refresh"   // Agent → Server: request proactive token rotation (new name)
	TypeAuthRequest    = "auth_request"    // DEPRECATED alias for TypeTokenRefresh (backward compat)
	TypeStreamBegin   = "stream_begin"
	TypeStreamEnd     = "stream_end"

	// TCP tunnel — Server → Agent: open a tunnel to a target host:port
	TypeTunnelOpen    = "tunnel_open"
	TypeTunnelClose   = "tunnel_close"
	TypeTunnelData    = "tunnel_data"    // bidirectional data frame
	TypeTunnelOpened  = "tunnel_opened"  // Agent → Server: tunnel established
	TypeTunnelClosed  = "tunnel_closed"  // Agent → Server: tunnel closed
	TypeTunnelError   = "tunnel_error"   // Agent → Server: tunnel error

	// Traffic sniffer — server-side handlers exist in tunnel.go but agent
	// doesn't implement them yet. SniffStart/Stop ARE used by the server's
	// handleAgentSniff/handleAgentSniffStop. SniffData/Started/Stopped are
	// reserved for future agent→server capture frames (not yet implemented).
	TypeSniffStart    = "sniff_start"
	TypeSniffStop     = "sniff_stop"

	// MITM TCP proxy — agent listens on a local port, forwards to target, logs all traffic
	TypeMitmStart    = "mitm_start"
	TypeMitmStop     = "mitm_stop"
	TypeMitmData     = "mitm_data"     // captured traffic frame (direction + hex data)
	TypeMitmStarted  = "mitm_started"
	TypeMitmStopped  = "mitm_stopped"

	// Debugger — attach to process, read memory, dump modules
	TypeDebugAttach   = "debug_attach"
	TypeDebugDetach   = "debug_detach"
	TypeDebugReadMem  = "debug_read_mem"
	TypeDebugModules  = "debug_modules"
	TypeDebugMemQuery = "debug_mem_query"

	// Process control (Server → Agent)
	TypeProcList      = "proc_list"
	TypeProcKill      = "proc_kill"
	TypeProcStart     = "proc_start"
	TypeProcListResult   = "proc_list_result"
	TypeProcKillResult   = "proc_kill_result"
	TypeProcStartResult  = "proc_start_result"

	// Results (Agent → Server)
	TypeExecResult          = "exec_result"
	TypeFSListResult       = "fs_list_result"
	TypeFSStatResult       = "fs_stat_result"
	TypeFSReadResult       = "fs_read_result"
	TypeFileSaveResult     = "file_save_result"
	TypeFileRemoveResult   = "file_remove_result"
	TypeFSMoveResult       = "fs_move_result"
	TypeFSMkdirResult      = "fs_mkdir_result"
	TypeFSHashResult       = "fs_hash_result"
	TypeCaptureResult      = "capture_result"
	TypeDisplayInfoResult  = "display_info_result"
	TypePointerClickResult = "pointer_click_result"
	TypeTextInputResult    = "text_input_result"
	TypeKeyPressResult     = "keypress_result"
	TypeKeyComboResult     = "keycombo_result"
	TypeHealthResult       = "health_result"
	TypeTaskListResult     = "task_list_result"
	TypeTaskStopResult     = "task_stop_result"
	TypeOpenLinkResult     = "open_link_result"
	TypeNotifyResult       = "notify_result"
	TypeClipboardReadResult  = "clipboard_read_result"
	TypeClipboardWriteResult = "clipboard_write_result"
	TypeTokenRotateResult   = "token_rotate_result" // new name
	TypeAuthRefreshResult   = "auth_refresh_result"  // DEPRECATED alias (backward compat)
	TypeStreamBeginResult   = "stream_begin_result"
	TypeStreamEndResult     = "stream_end_result"
	TypeError               = "error"
)

// Error codes
const (
	ErrTimeout           = "TIMEOUT"
	ErrPermissionDenied  = "PERMISSION_DENIED"
	ErrNotFound          = "NOT_FOUND"
	ErrInvalidParams     = "INVALID_PARAMS"
	ErrPlatformNotSupported = "PLATFORM_NOT_SUPPORTED"
	ErrInternal          = "INTERNAL_ERROR"
)

// Envelope is the top-level message wrapper.
type Envelope struct {
	ID     string          `json:"id"`
	Type   string          `json:"type"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *ErrorInfo      `json:"error,omitempty"`
}

// ErrorInfo represents an error response.
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// --- Agent Info (sent on connect) ---
type AgentInfo struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	Mode            string `json:"mode"` // "outbound", "inbound", "dual"
	ProtocolVersion string `json:"protocol_version,omitempty"` // "1" = original, "2" = post-refactor. Missing = "1" (old agent).
}

// --- Command params ---
type ExecParams struct {
	Command string            `json:"command"`
	Timeout int               `json:"timeout,omitempty"` // seconds, default 60
	WorkDir string            `json:"workdir,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type ExecResult struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
	TimedOut   bool   `json:"timed_out"`
}

type FSParams struct {
	Path   string `json:"path"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Data   string `json:"data,omitempty"`   // base64 for writes
	Mode   string `json:"mode,omitempty"`   // octal string for writes
	From   string `json:"from,omitempty"`   // for moves
	To     string `json:"to,omitempty"`     // for moves
}

type FSEntry struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime int64  `json:"mod_time"`
	IsDir   bool   `json:"is_dir"`
}

type FSListResult struct {
	Entries []FSEntry `json:"entries"`
}

type FSStatResult struct {
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime int64  `json:"mod_time"`
	IsDir   bool   `json:"is_dir"`
	Exists  bool   `json:"exists"`
}

type FSReadResult struct {
	Data     string `json:"data"` // base64
	Size     int64  `json:"size"`
	Encoding string `json:"encoding"`
}

type FSWriteResult struct {
	Written int    `json:"written"`
	Path    string `json:"path"`
}

type FSDeleteResult struct {
	Deleted bool   `json:"deleted"`
	Path    string `json:"path"`
}

type FSMoveResult struct {
	Moved bool   `json:"moved"`
	From  string `json:"from"`
	To    string `json:"to"`
}

type FSMkdirResult struct {
	Created bool   `json:"created"`
	Path    string `json:"path"`
}

type FSHashResult struct {
	Path string `json:"path"`
	Hash string `json:"sha256"`
	Size int64  `json:"size"`
}

type ScreenParams struct {
	Display int `json:"display,omitempty"`
	Quality int `json:"quality,omitempty"`
}

type ScreenRegionParams struct {
	X       int `json:"x"`
	Y       int `json:"y"`
	Width   int `json:"width"`
	Height  int `json:"height"`
	Display int `json:"display,omitempty"`
}

type CaptureResult struct {
	Format    string `json:"format"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Data      string `json:"data"` // base64
	SizeBytes int64  `json:"size_bytes"`
}

type ScreenInfo struct {
	Displays []DisplayInfo `json:"displays"`
}

type DisplayInfo struct {
	ID        int  `json:"id"`
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	Scale     float64 `json:"scale"`
	IsPrimary bool   `json:"is_primary"`
}

type ScreenStreamParams struct {
	Display int `json:"display,omitempty"`
	FPS     int `json:"fps,omitempty"`
	Quality int `json:"quality,omitempty"`
}

type ScreenStreamStartResult struct {
	StreamID string `json:"stream_id"`
	Port     int    `json:"port,omitempty"`
}

type ScreenStreamStopParams struct {
	StreamID string `json:"stream_id"`
}

type InputParams struct {
	X      int    `json:"x,omitempty"`
	Y      int    `json:"y,omitempty"`
	Button string `json:"button,omitempty"` // left, right, middle
	Text   string `json:"text,omitempty"`
	Key    string `json:"key,omitempty"`
	Keys   []string `json:"keys,omitempty"`
}

type InputResult struct {
	Success bool `json:"success"`
}

type HealthResult struct {
	AgentVersion   string  `json:"agent_version"`
	Hostname       string  `json:"hostname"`
	OS             string  `json:"os"`
	Arch           string  `json:"arch"`
	UptimeSeconds  int64   `json:"uptime_seconds"`
	Mode           string  `json:"mode"`
	ConnectedSince string  `json:"connected_since"`
	// Resource usage (populated by agents that report it; zero = not reported).
	CPUPercent float64 `json:"cpu_percent,omitempty"`
	MemoryMB   float64 `json:"memory_mb,omitempty"`
	DiskFreeMB float64 `json:"disk_free_mb,omitempty"`
}

type ProcessInfo struct {
	PID        int     `json:"pid"`
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpu_percent"`
	MemoryMB   float64 `json:"memory_mb"`
}

type ProcessListResult struct {
	Processes []ProcessInfo `json:"processes"`
}

type TaskStopParams struct {
	PID    int `json:"pid"`
	Signal int `json:"signal,omitempty"`
}

type TaskStopResult struct {
	Killed bool `json:"killed"`
	PID    int  `json:"pid"`
}

type OpenURLParams struct {
	URL string `json:"url"`
}

type NotifyParams struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Icon  string `json:"icon,omitempty"`
}

type ClipboardResult struct {
	Text string `json:"text"`
}

type ClipboardWriteParams struct {
	Text string `json:"text"`
}

type TokenRotateParams struct {
	NewToken string    `json:"new_token"`
	Expiry   time.Time `json:"expiry,omitempty"` // zero = no expiry
}

// TokenRotateResult is the agent's response confirming a token rotation.
type TokenRotateResult struct {
	Rotated  bool   `json:"rotated"`
	NewToken string `json:"new_token,omitempty"` // echo back for confirmation
}

// TunnelParams opens a TCP tunnel from the agent to a target host:port.
// The server creates a local TCP listener; connections to it are relayed
// through the WebSocket to the agent, which connects to the target.
type TunnelParams struct {
	TargetHost string `json:"target_host"` // e.g. "127.0.0.1"
	TargetPort int    `json:"target_port"` // e.g. 1234
	ListenPort int    `json:"listen_port,omitempty"` // server-side listen port (0 = auto)
}

type TunnelOpenResult struct {
	ListenPort int    `json:"listen_port"`
	TunnelID   string `json:"tunnel_id"`
}

type TunnelDataParams struct {
	TunnelID  string `json:"tunnel_id"`
	Direction string `json:"direction"` // "client→target" or "target→client"
	Data      string `json:"data"`      // base64
}

type TunnelCloseParams struct {
	TunnelID string `json:"tunnel_id"`
}

// SniffParams starts a traffic sniffer that connects to target and relays
// all traffic, capturing it for inspection. Similar to tunnel but with logging.
type SniffParams struct {
	TargetHost string `json:"target_host"`
	TargetPort int    `json:"target_port"`
	Duration   int    `json:"duration,omitempty"` // seconds, 0 = until stopped
}

type SniffStartResult struct {
	SniffID  string `json:"sniff_id"`
	Captures int    `json:"captures"` // number of frames captured
}

// MitmStartParams starts a MITM TCP proxy on the agent.
type MitmStartParams struct {
	ListenAddr string `json:"listen_addr"` // e.g. "127.0.0.3:1516"
	TargetAddr string `json:"target_addr"` // e.g. "127.0.0.1:1516"
	LogPath    string `json:"log_path"`    // e.g. "C:\	emp\\mitm-traffic.log"
	ReuseAddr  bool   `json:"reuse_addr,omitempty"` // use SO_REUSEADDR (Windows)
}

type MitmStartResult struct {
	MitmID     string `json:"mitm_id"`
	ListenAddr string `json:"listen_addr"`
}

type MitmStopParams struct {
	MitmID string `json:"mitm_id"`
}

type MitmTrafficResult struct {
	MitmID    string `json:"mitm_id"`
	Traffic   string `json:"traffic"`    // full hex log content
	Size      int    `json:"size"`
	Entries   int    `json:"entries"`    // number of C->T / T->C entries
}

// --- Debugger params/results ---

type DebugAttachParams struct {
	PID         int    `json:"pid"`
	ProcessName string `json:"process_name,omitempty"` // alternative: find by name
}

type DebugAttachResult struct {
	DebugID  string `json:"debug_id"`
	PID      int    `json:"pid"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	BaseAddr uint64 `json:"base_addr"`
}

type DebugReadMemParams struct {
	DebugID string `json:"debug_id"`
	Address uint64 `json:"address"`
	Size    int    `json:"size"`
}

type DebugReadMemResult struct {
	Data    string `json:"data"`     // base64 encoded
	HexData string `json:"hex_data"` // hex string for readability
	Size    int    `json:"size"`
	Address uint64 `json:"address"`
}

type DebugModulesParams struct {
	DebugID string `json:"debug_id"`
}

type DebugModuleInfo struct {
	Name     string `json:"name"`
	BaseAddr uint64 `json:"base_addr"`
	Size     int    `json:"size"`
	Path     string `json:"path"`
}

type DebugModulesResult struct {
	Modules []DebugModuleInfo `json:"modules"`
}

type DebugMemQueryParams struct {
	DebugID string `json:"debug_id"`
	Address uint64 `json:"address"`
}

type DebugMemRegion struct {
	BaseAddress uint64 `json:"base_address"`
	Size        uint64 `json:"size"`
	State       uint32 `json:"state"`
	Protect     uint32 `json:"protect"`
	Type        uint32 `json:"type"`
}

type DebugMemQueryResult struct {
	Region DebugMemRegion `json:"region"`
}

type DebugDetachParams struct {
	DebugID string `json:"debug_id"`
}

// ProcStartParams starts a process on the agent.
type ProcStartParams struct {
	Command string            `json:"command"`
	WorkDir string            `json:"workdir,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Background bool           `json:"background,omitempty"` // don't wait for completion
}

type ProcStartResult struct {
	PID      int    `json:"pid,omitempty"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode int    `json:"exit_code"`
}

// NewError creates an error envelope.
func NewError(id string, code string, message string) Envelope {
	return Envelope{
		ID:   id,
		Type: TypeError,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
	}
}

// NewPing creates a ping message.
func NewPing() Envelope {
	return Envelope{ID: fmt.Sprintf("ping-%d", nowMillis()), Type: TypePing}
}

// NewPong creates a pong message.
func NewPong(id string) Envelope {
	return Envelope{ID: id, Type: TypePong}
}

// NewResult creates a result envelope with marshalled result.
func NewResult(id string, resultType string, result interface{}) Envelope {
	data, err := json.Marshal(result)
	if err != nil {
		return NewError(id, ErrInternal, fmt.Sprintf("failed to marshal result: %v", err))
	}
	return Envelope{
		ID:     id,
		Type:   resultType,
		Result: data,
	}
}

// ParseCommand parses a command envelope's params into a typed struct.
func ParseCommand[T any](env Envelope) (T, error) {
	var params T
	if err := json.Unmarshal(env.Params, &params); err != nil {
		return params, fmt.Errorf("parse params: %w", err)
	}
	return params, nil
}

func nowMillis() int64 {
	return time.Now().UnixMilli()
}