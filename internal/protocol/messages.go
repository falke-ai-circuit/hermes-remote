package protocol

import (
	"encoding/json"
	"fmt"
)

// Message types
const (
	// Agent → Server
	TypePing  = "ping"
	TypePong  = "pong"

	// Commands (Server → Agent)
	TypeShell        = "shell"
	TypeFSList       = "fs_list"
	TypeFSStat       = "fs_stat"
	TypeFSRead       = "fs_read"
	TypeFSWrite      = "fs_write"
	TypeFSDelete     = "fs_delete"
	TypeFSMove       = "fs_move"
	TypeFSMkdir      = "fs_mkdir"
	TypeScreenshot   = "screenshot"
	TypeScreenRegion = "screen_region"
	TypeScreenInfo   = "screen_info"
	TypeClick        = "click"
	TypeType         = "type"
	TypeKey          = "key"
	TypeHotkey       = "hotkey"
	TypeHealth       = "health"
	TypeProcessList  = "process_list"
	TypeProcessKill  = "process_kill"
	TypeOpenURL      = "open_url"
	TypeNotify       = "notify"
	TypeClipboardGet = "clipboard_get"
	TypeClipboardSet = "clipboard_set"
	TypeTokenRotate  = "token_rotate"
	TypeStreamStart  = "stream_start"
	TypeStreamStop   = "stream_stop"

	// Results (Agent → Server)
	TypeShellResult       = "shell_result"
	TypeFSListResult      = "fs_list_result"
	TypeFSStatResult      = "fs_stat_result"
	TypeFSReadResult      = "fs_read_result"
	TypeFSWriteResult     = "fs_write_result"
	TypeFSDeleteResult    = "fs_delete_result"
	TypeFSMoveResult      = "fs_move_result"
	TypeFSMkdirResult     = "fs_mkdir_result"
	TypeScreenshotResult  = "screenshot_result"
	TypeScreenInfoResult  = "screen_info_result"
	TypeClickResult       = "click_result"
	TypeTypeResult        = "type_result"
	TypeKeyResult         = "key_result"
	TypeHotkeyResult      = "hotkey_result"
	TypeHealthResult      = "health_result"
	TypeProcessListResult = "process_list_result"
	TypeProcessKillResult = "process_kill_result"
	TypeOpenURLResult     = "open_url_result"
	TypeNotifyResult      = "notify_result"
	TypeClipboardGetResult = "clipboard_get_result"
	TypeClipboardSetResult = "clipboard_set_result"
	TypeTokenRotateResult  = "token_rotate_result"
	TypeStreamStartResult  = "stream_start_result"
	TypeStreamStopResult   = "stream_stop_result"
	TypeError             = "error"
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
	Name    string `json:"name"`
	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	Mode    string `json:"mode"` // "outbound", "inbound", "dual"
}

// --- Command params ---
type ShellParams struct {
	Command string            `json:"command"`
	Timeout int               `json:"timeout,omitempty"` // seconds, default 60
	WorkDir string            `json:"workdir,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type ShellResult struct {
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

type ScreenshotResult struct {
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
	AgentVersion   string `json:"agent_version"`
	Hostname       string `json:"hostname"`
	OS             string `json:"os"`
	Arch           string `json:"arch"`
	UptimeSeconds  int64  `json:"uptime_seconds"`
	Mode           string `json:"mode"`
	ConnectedSince string `json:"connected_since"`
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

type ProcessKillParams struct {
	PID    int `json:"pid"`
	Signal int `json:"signal,omitempty"`
}

type ProcessKillResult struct {
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

type ClipboardSetParams struct {
	Text string `json:"text"`
}

type TokenRotateParams struct {
	NewToken string `json:"new_token"`
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
	return 0 // placeholder — real impl uses time.Now().UnixMilli()
}
