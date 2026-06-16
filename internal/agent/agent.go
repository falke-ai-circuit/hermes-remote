package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/falke-ai-circuit/hermes-remote/internal/platform"
	"github.com/falke-ai-circuit/hermes-remote/internal/protocol"
	"github.com/gorilla/websocket"
)

const (
	pingInterval    = 15 * time.Second
	missThreshold   = 3
	defaultTimeout  = 60
	maxTimeout      = 300
)

// Config holds agent configuration.
type Config struct {
	Mode     string // "outbound", "inbound", "dual"
	URL      string // wss://host:port for outbound
	Addr     string // :port for inbound
	Token    string
	CertPath string // CA cert for outbound (server verification)
	// ClientCertFile/ClientKeyFile are an optional client certificate used for
	// TLS mutual authentication when dialing a wss:// server (mTLS).
	ClientCertFile string // client cert for mTLS (outbound)
	ClientKeyFile  string // client key for mTLS (outbound)
	CertFile       string // TLS cert for inbound
	KeyFile        string // TLS key for inbound
	Name           string // optional display name
	LogPath        string // log file path (empty = stdout)
	MaxRetries     int           // 0 = infinite retries
	BackoffMin     time.Duration // default 1s
	BackoffMax     time.Duration // default 60s
	TokenFile      string        // path to persist token (empty = no persistence)
}

// Agent is the remote agent instance.
type Agent struct {
	cfg            Config
	conn           *websocket.Conn
	connectedAt    time.Time
	mu             sync.Mutex
	lastPing       time.Time
	pingMisses     int
	backoffAttempt int
	plat           platform.Platform
	server         *protocol.Server
	stopped        chan struct{}

	// tokenExpiry is the expiry time of the current token, if the server has
	// issued a rotating token with an expiry. A zero value means "no expiry".
	// Guarded by mu alongside cfg.Token.
	tokenExpiry time.Time
}

// New creates a new agent.
func New(cfg Config) *Agent {
	return &Agent{
		cfg:     cfg,
		stopped: make(chan struct{}),
		plat:    platform.New(cfg.Name),
	}
}

// Run starts the agent in the configured mode.
func (a *Agent) Run() error {
	switch a.cfg.Mode {
	case "outbound":
		return a.runOutbound()
	case "inbound":
		return a.runInbound()
	case "dual":
		go a.runInbound()
		return a.runOutbound()
	default:
		return fmt.Errorf("unknown mode: %s", a.cfg.Mode)
	}
}

func (a *Agent) runOutbound() error {
	log.Printf("[agent] connecting to %s (mode: outbound)", a.cfg.URL)
	for {
		conn, err := protocol.Dial(a.cfg.URL, a.cfg.CertPath, a.cfg.ClientCertFile, a.cfg.ClientKeyFile, a.cfg.Token)
		if err != nil {
			a.backoffAttempt++
			if a.cfg.MaxRetries > 0 && a.backoffAttempt > a.cfg.MaxRetries {
				return fmt.Errorf("max retries (%d) exceeded: %w", a.cfg.MaxRetries, err)
			}
			backoff := a.computeBackoff()
			log.Printf("[agent] connection failed (attempt %d): %v, retrying in %v", a.backoffAttempt, err, backoff)
			select {
			case <-a.stopped:
				return nil
			case <-time.After(backoff):
			}
			continue
		}
		log.Printf("[agent] connected to %s", a.cfg.URL)
		a.mu.Lock()
		a.conn = conn
		a.connectedAt = time.Now()
		a.pingMisses = 0
		a.backoffAttempt = 0
		a.mu.Unlock()
		a.handleConnection(conn)
		// disconnected — reconnect
		log.Printf("[agent] disconnected, reconnecting...")
		select {
		case <-a.stopped:
			return nil
		default:
		}
	}
}

// computeBackoff returns an exponential backoff duration with jitter.
// Formula: min * 2^(attempt-1) capped at max, plus random jitter.
func (a *Agent) computeBackoff() time.Duration {
	min := a.cfg.BackoffMin
	max := a.cfg.BackoffMax
	if min <= 0 {
		min = 1 * time.Second
	}
	if max <= 0 {
		max = 60 * time.Second
	}
	// Cap the exponent to avoid overflow: 2^10 = 1024 is plenty
	exp := a.backoffAttempt - 1
	if exp > 10 {
		exp = 10
	}
	base := min * time.Duration(1<<exp)
	if base > max {
		base = max
	}
	// Add jitter: random value in [0, base/2]
	jitter := time.Duration(rand.Int64N(int64(base/2 + 1)))
	return base + jitter
}

func (a *Agent) runInbound() error {
	log.Printf("[agent] listening on %s (mode: inbound)", a.cfg.Addr)
	srv, err := protocol.NewServer(a.cfg.Addr, a.cfg.CertFile, a.cfg.KeyFile, "")
	if err != nil {
		return fmt.Errorf("start server: %w", err)
	}
	a.server = srv
	srv.OnConnect = func(conn *websocket.Conn, r *http.Request) bool {
		token := r.Header.Get("Authorization")
		if token != "Bearer "+a.cfg.Token {
			log.Printf("[agent] rejected connection: bad token from %s", r.RemoteAddr)
			return false
		}
		log.Printf("[agent] accepted connection from %s", r.RemoteAddr)
		a.mu.Lock()
		a.conn = conn
		a.connectedAt = time.Now()
		a.pingMisses = 0
		a.mu.Unlock()
		go a.handleConnection(conn)
		return true
	}
	log.Printf("[agent] server shut down")
	return srv.ListenAndServe()
}

func (a *Agent) handleConnection(conn *websocket.Conn) {
	defer conn.Close()

	// Send agent info
	info := protocol.AgentInfo{
		Name:    a.cfg.Name,
		Version: Version,
		OS:      getOS(),
		Arch:    getArch(),
		Mode:    a.cfg.Mode,
	}
	if err := protocol.WriteMessage(conn, protocol.Envelope{
		ID:     "agent-info",
		Type:   "agent_info",
		Result: mustMarshal(info),
	}); err != nil {
		log.Printf("[agent] failed to send agent info: %v", err)
		return
	}

	// Ping ticker
	pingTicker := time.NewTicker(pingInterval)
	defer pingTicker.Stop()

	// Token-refresh ticker: checks every minute whether the current token is
	// close to expiry and, if so, asks the server for a new one proactively.
	refreshInterval := 60 * time.Second
	refreshTicker := time.NewTicker(refreshInterval)
	defer refreshTicker.Stop()
	// refreshLeadTime is how far before expiry the agent requests a new token.
	const refreshLeadTime = 5 * time.Minute

	// Read messages
	readErr := make(chan error, 1)
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				readErr <- err
				return
			}
			var env protocol.Envelope
			if err := json.Unmarshal(msg, &env); err != nil {
				resp := protocol.NewError("", protocol.ErrInvalidParams, "invalid JSON")
				protocol.WriteMessage(conn, resp)
				continue
			}
			a.handleCommand(conn, env)
		}
	}()

	for {
		select {
		case err := <-readErr:
			log.Printf("[agent] read error: %v", err)
			return
		case <-pingTicker.C:
			a.mu.Lock()
			a.pingMisses++
			if a.pingMisses >= missThreshold {
				log.Printf("[agent] ping timeout (%d misses), closing", a.pingMisses)
				a.mu.Unlock()
				return
			}
			a.mu.Unlock()
			if err := protocol.WriteMessage(conn, protocol.Envelope{
				ID:   fmt.Sprintf("ping-%d", time.Now().UnixMilli()),
				Type: protocol.TypePing,
			}); err != nil {
				log.Printf("[agent] ping failed: %v", err)
				return
			}
		case <-refreshTicker.C:
			// Proactive refresh: if the token has an expiry and we're within
			// refreshLeadTime of it, ask the server to rotate the token now.
			a.mu.Lock()
			expiry := a.tokenExpiry
			a.mu.Unlock()
			if !expiry.IsZero() && time.Now().Add(refreshLeadTime).After(expiry) {
				log.Printf("[agent] token nearing expiry (%v), requesting refresh", expiry)
				refreshEnv := protocol.Envelope{
					ID:   fmt.Sprintf("token-refresh-%d", time.Now().UnixMilli()),
					Type: protocol.TypeTokenRefresh,
				}
				if err := protocol.WriteMessage(conn, refreshEnv); err != nil {
					log.Printf("[agent] token refresh request failed: %v", err)
				}
			}
		}
	}
}

func (a *Agent) handleCommand(conn *websocket.Conn, env protocol.Envelope) {
	var resp protocol.Envelope
	switch env.Type {
	case protocol.TypePing:
		resp = protocol.NewPong(env.ID)
	case "shell":
		resp = a.handleShell(env)
	case "shell_pty":
		resp = a.handleShellPTY(env)
	case "fs_list":
		resp = a.handleFSList(env)
	case "fs_stat":
		resp = a.handleFSStat(env)
	case "fs_read":
		resp = a.handleFSRead(env)
	case "fs_write":
		resp = a.handleFSWrite(env)
	case "fs_delete":
		resp = a.handleFSDelete(env)
	case "fs_move":
		resp = a.handleFSMove(env)
	case "fs_mkdir":
		resp = a.handleFSMkdir(env)
	case "screenshot":
		resp = a.handleScreenshot(env)
	case "screen_info":
		resp = a.handleScreenInfo(env)
	case "click":
		resp = a.handleClick(env)
	case "type":
		resp = a.handleType(env)
	case "key":
		resp = a.handleKey(env)
	case "hotkey":
		resp = a.handleHotkey(env)
	case "health":
		resp = a.handleHealth(env)
	case "process_list":
		resp = a.handleProcessList(env)
	case "process_kill":
		resp = a.handleProcessKill(env)
	case "open_url":
		resp = a.handleOpenURL(env)
	case "notify":
		resp = a.handleNotify(env)
	case "clipboard_get":
		resp = a.handleClipboardGet(env)
	case "clipboard_set":
		resp = a.handleClipboardSet(env)
	case "token_rotate":
		resp = a.handleTokenRotate(env)
	default:
		resp = protocol.NewError(env.ID, protocol.ErrInvalidParams, fmt.Sprintf("unknown command: %s", env.Type))
	}
	if err := protocol.WriteMessage(conn, resp); err != nil {
		log.Printf("[agent] write error: %v", err)
	}
}

func (a *Agent) handleShell(env protocol.Envelope) protocol.Envelope {
	params, err := protocol.ParseCommand[protocol.ShellParams](env)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}
	if params.Timeout <= 0 {
		params.Timeout = defaultTimeout
	}
	if params.Timeout > maxTimeout {
		params.Timeout = maxTimeout
	}
	start := time.Now()
	result, err := a.plat.Exec(params.Command, params.Timeout, params.WorkDir, params.Env)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, err.Error())
	}
	result.DurationMs = time.Since(start).Milliseconds()
	return protocol.NewResult(env.ID, protocol.TypeShellResult, result)
}

func (a *Agent) handleShellPTY(env protocol.Envelope) protocol.Envelope {
	return a.handleShell(env)
}

// --- command handlers for all 25 protocol commands ---

func (a *Agent) handleFSList(env protocol.Envelope) protocol.Envelope {
	params, err := protocol.ParseCommand[protocol.FSParams](env)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}
	entries, err := a.plat.ListDir(params.Path)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, err.Error())
	}
	return protocol.NewResult(env.ID, protocol.TypeFSListResult, protocol.FSListResult{Entries: entries})
}

func (a *Agent) handleFSStat(env protocol.Envelope) protocol.Envelope {
	params, err := protocol.ParseCommand[protocol.FSParams](env)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}
	result, err := a.plat.FileStat(params.Path)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, err.Error())
	}
	return protocol.NewResult(env.ID, protocol.TypeFSStatResult, result)
}

func (a *Agent) handleFSRead(env protocol.Envelope) protocol.Envelope {
	params, err := protocol.ParseCommand[protocol.FSParams](env)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}
	result, err := a.plat.ReadFile(params.Path, params.Offset, params.Limit)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrNotFound, err.Error())
	}
	return protocol.NewResult(env.ID, protocol.TypeFSReadResult, result)
}

func (a *Agent) handleFSWrite(env protocol.Envelope) protocol.Envelope {
	params, err := protocol.ParseCommand[protocol.FSParams](env)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}
	data, err := decodeBase64(params.Data)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, "invalid base64 data")
	}
	result, err := a.plat.WriteFile(params.Path, data, params.Mode)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, err.Error())
	}
	return protocol.NewResult(env.ID, protocol.TypeFSWriteResult, result)
}

func (a *Agent) handleFSDelete(env protocol.Envelope) protocol.Envelope {
	params, err := protocol.ParseCommand[protocol.FSParams](env)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}
	result, err := a.plat.DeleteFile(params.Path)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, err.Error())
	}
	return protocol.NewResult(env.ID, protocol.TypeFSDeleteResult, result)
}

func (a *Agent) handleFSMove(env protocol.Envelope) protocol.Envelope {
	params, err := protocol.ParseCommand[protocol.FSParams](env)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}
	result, err := a.plat.MoveFile(params.From, params.To)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, err.Error())
	}
	return protocol.NewResult(env.ID, protocol.TypeFSMoveResult, result)
}

func (a *Agent) handleFSMkdir(env protocol.Envelope) protocol.Envelope {
	params, err := protocol.ParseCommand[protocol.FSParams](env)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}
	result, err := a.plat.Mkdir(params.Path)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, err.Error())
	}
	return protocol.NewResult(env.ID, protocol.TypeFSMkdirResult, result)
}

func (a *Agent) handleScreenshot(env protocol.Envelope) protocol.Envelope {
	params, err := protocol.ParseCommand[protocol.ScreenParams](env)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}
	result, err := a.plat.Screenshot(params.Display, params.Quality)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, err.Error())
	}
	return protocol.NewResult(env.ID, protocol.TypeScreenshotResult, result)
}

func (a *Agent) handleScreenInfo(env protocol.Envelope) protocol.Envelope {
	return protocol.NewResult(env.ID, protocol.TypeScreenInfoResult, a.plat.ScreenInfo())
}

func (a *Agent) handleClick(env protocol.Envelope) protocol.Envelope {
	params, err := protocol.ParseCommand[protocol.InputParams](env)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}
	if err := a.plat.Click(params.X, params.Y, params.Button); err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, err.Error())
	}
	return protocol.NewResult(env.ID, protocol.TypeClickResult, protocol.InputResult{Success: true})
}

func (a *Agent) handleType(env protocol.Envelope) protocol.Envelope {
	params, err := protocol.ParseCommand[protocol.InputParams](env)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}
	if err := a.plat.TypeText(params.Text); err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, err.Error())
	}
	return protocol.NewResult(env.ID, protocol.TypeTypeResult, protocol.InputResult{Success: true})
}

func (a *Agent) handleKey(env protocol.Envelope) protocol.Envelope {
	params, err := protocol.ParseCommand[protocol.InputParams](env)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}
	if err := a.plat.KeyPress(params.Key); err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, err.Error())
	}
	return protocol.NewResult(env.ID, protocol.TypeKeyResult, protocol.InputResult{Success: true})
}

func (a *Agent) handleHotkey(env protocol.Envelope) protocol.Envelope {
	params, err := protocol.ParseCommand[protocol.InputParams](env)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}
	if err := a.plat.Hotkey(params.Keys); err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, err.Error())
	}
	return protocol.NewResult(env.ID, protocol.TypeHotkeyResult, protocol.InputResult{Success: true})
}

func (a *Agent) handleHealth(env protocol.Envelope) protocol.Envelope {
	return protocol.NewResult(env.ID, protocol.TypeHealthResult, a.plat.Health(a.cfg.Mode))
}

func (a *Agent) handleProcessList(env protocol.Envelope) protocol.Envelope {
	procs, err := a.plat.ProcessList()
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, err.Error())
	}
	return protocol.NewResult(env.ID, protocol.TypeProcessListResult, protocol.ProcessListResult{Processes: procs})
}

func (a *Agent) handleProcessKill(env protocol.Envelope) protocol.Envelope {
	params, err := protocol.ParseCommand[protocol.ProcessKillParams](env)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}
	if err := a.plat.ProcessKill(params.PID, params.Signal); err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, err.Error())
	}
	return protocol.NewResult(env.ID, protocol.TypeProcessKillResult, protocol.ProcessKillResult{Killed: true, PID: params.PID})
}

func (a *Agent) handleOpenURL(env protocol.Envelope) protocol.Envelope {
	params, err := protocol.ParseCommand[protocol.OpenURLParams](env)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}
	if err := a.plat.OpenURL(params.URL); err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, err.Error())
	}
	return protocol.NewResult(env.ID, protocol.TypeOpenURLResult, protocol.InputResult{Success: true})
}

func (a *Agent) handleNotify(env protocol.Envelope) protocol.Envelope {
	params, err := protocol.ParseCommand[protocol.NotifyParams](env)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}
	if err := a.plat.Notify(params.Title, params.Body, params.Icon); err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, err.Error())
	}
	return protocol.NewResult(env.ID, protocol.TypeNotifyResult, protocol.InputResult{Success: true})
}

func (a *Agent) handleClipboardGet(env protocol.Envelope) protocol.Envelope {
	text, err := a.plat.ClipboardGet()
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, err.Error())
	}
	return protocol.NewResult(env.ID, protocol.TypeClipboardGetResult, protocol.ClipboardResult{Text: text})
}

func (a *Agent) handleClipboardSet(env protocol.Envelope) protocol.Envelope {
	params, err := protocol.ParseCommand[protocol.ClipboardSetParams](env)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}
	if err := a.plat.ClipboardSet(params.Text); err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, err.Error())
	}
	return protocol.NewResult(env.ID, protocol.TypeClipboardSetResult, protocol.InputResult{Success: true})
}

func (a *Agent) handleTokenRotate(env protocol.Envelope) protocol.Envelope {
	params, err := protocol.ParseCommand[protocol.TokenRotateParams](env)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}
	a.mu.Lock()
	oldToken := a.cfg.Token
	a.cfg.Token = params.NewToken
	// Track the expiry so the proactive-refresh loop can request a new token
	// before this one expires. A zero Expiry means the server did not set one.
	a.tokenExpiry = params.Expiry
	a.mu.Unlock()

	// Persist the new token to disk so reconnects use the rotated token.
	if a.cfg.TokenFile != "" {
		if err := a.persistToken(params.NewToken); err != nil {
			log.Printf("[agent] failed to persist rotated token: %v", err)
		} else {
			log.Printf("[agent] persisted rotated token to %s", a.cfg.TokenFile)
		}
	}

	log.Printf("[agent] token rotated (old len=%d, new len=%d)", len(oldToken), len(params.NewToken))
	return protocol.NewResult(env.ID, protocol.TypeTokenRotateResult, protocol.TokenRotateResult{
		Rotated:  true,
		NewToken: params.NewToken,
	})
}

// persistToken writes the token to the configured TokenFile with 0600 perms.
// It is called by handleTokenRotate after a successful rotation so reconnects
// pick up the new token automatically.
func (a *Agent) persistToken(token string) error {
	if a.cfg.TokenFile == "" {
		return nil
	}
	return os.WriteFile(a.cfg.TokenFile, []byte(token), 0600)
}

// LoadPersistedToken reads a previously persisted token from TokenFile. It
// returns an empty string (and no error) if the file does not exist or is
// empty. Used at startup to resume with the most recent rotated token.
func LoadPersistedToken(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func decodeBase64(s string) ([]byte, error) {
	// Simple decoder inline
	if s == "" {
		return nil, nil
	}
	var result []byte
	table := map[byte]byte{}
	for i := 0; i < 64; i++ {
		c := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"[i]
		table[c] = byte(i)
	}
	data := []byte(s)
	// Strip padding
	for len(data) > 0 && data[len(data)-1] == '=' {
		data = data[:len(data)-1]
	}
	for i := 0; i < len(data); i += 4 {
		if i+1 >= len(data) {
			break
		}
		b0 := table[data[i]]
		b1 := table[data[i+1]]
		b2 := byte(0)
		b3 := byte(0)
		if i+2 < len(data) {
			b2 = table[data[i+2]]
		}
		if i+3 < len(data) {
			b3 = table[data[i+3]]
		}
		result = append(result, (b0<<2)|(b1>>4))
		if i+2 < len(data) {
			result = append(result, ((b1&0x0f)<<4)|(b2>>2))
		}
		if i+3 < len(data) {
			result = append(result, ((b2&0x03)<<6)|b3)
		}
	}
	return result, nil
}

// SendPrompt sends a shell command to the connected server and displays results.
func (a *Agent) SendPrompt(prompt string) {
	a.mu.Lock()
	conn := a.conn
	a.mu.Unlock()
	if conn == nil {
		log.Printf("[agent] not connected, cannot send prompt")
		return
	}
	env := protocol.Envelope{
		ID:   fmt.Sprintf("prompt-%d", time.Now().UnixMilli()),
		Type: "shell",
		Params: mustMarshal(protocol.ShellParams{
			Command: prompt,
			Timeout: defaultTimeout,
		}),
	}
	if err := protocol.WriteMessage(conn, env); err != nil {
		log.Printf("[agent] send prompt error: %v", err)
	}
}

// Version is the agent version.
const Version = "0.1.0"

func getOS() string   { return runtime.GOOS }
func getArch() string { return runtime.GOARCH }
