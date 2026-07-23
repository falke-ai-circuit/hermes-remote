package agent

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/falke-ai-circuit/probe/internal/protocol"
)

type mitmSession struct {
	id          string
	listenAddr  string
	targetAddr  string
	logPath     string
	listener    net.Listener
	trafficLog  strings.Builder
	trafficMu   sync.Mutex
	connections int
	stop        chan struct{}
}

type mitmManager struct {
	mu       sync.Mutex
	sessions map[string]*mitmSession
	nextID   int
}

func newMitmManager() *mitmManager {
	return &mitmManager{sessions: make(map[string]*mitmSession)}
}

func (a *Agent) handleMitmStart(env protocol.Envelope) protocol.Envelope {
	var params protocol.MitmStartParams
	if err := json.Unmarshal(env.Params, &params); err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}
	if params.ListenAddr == "" || params.TargetAddr == "" {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, "listen_addr and target_addr required")
	}

	// Create listener with optional SO_REUSEADDR
	lc := net.ListenConfig{}
	if params.ReuseAddr {
		lc.Control = func(network, address string, conn syscall.RawConn) error {
			var sockErr error
			err := conn.Control(func(fd uintptr) {
				sockErr = setReuseAddr(fd)
			})
			if err != nil {
				return err
			}
			return sockErr
		}
	}

	ln, err := lc.Listen(context.Background(), "tcp", params.ListenAddr)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInternal, fmt.Sprintf("listen failed: %v", err))
	}

	a.mitmMgr.mu.Lock()
	a.mitmMgr.nextID++
	id := fmt.Sprintf("mitm-%d", a.mitmMgr.nextID)
	a.mitmMgr.mu.Unlock()

	session := &mitmSession{
		id:         id,
		listenAddr: params.ListenAddr,
		targetAddr: params.TargetAddr,
		logPath:    params.LogPath,
		listener:   ln,
		stop:       make(chan struct{}),
	}

	// Write log file header if path provided
	if params.LogPath != "" {
		f, err := os.OpenFile(params.LogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err == nil {
			fmt.Fprintf(f, "MITM Proxy started at %s\n", time.Now().Format(time.RFC3339))
			fmt.Fprintf(f, "Listen: %s -> Target: %s\n\n", params.ListenAddr, params.TargetAddr)
			f.Close()
		}
	}

	a.mitmMgr.mu.Lock()
	a.mitmMgr.sessions[id] = session
	a.mitmMgr.mu.Unlock()

	go session.acceptLoop()

	return protocol.NewResult(env.ID, protocol.TypeMitmStarted, map[string]interface{}{
		"mitm_id":     id,
		"listen_addr": params.ListenAddr,
	})
}

func (s *mitmSession) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stop:
				return
			default:
				log.Printf("[mitm %s] accept error: %v", s.id, err)
				return
			}
		}
		s.connections++
		go s.handleConn(conn)
	}
}

func (s *mitmSession) handleConn(client net.Conn) {
	defer client.Close()

	target, err := net.DialTimeout("tcp", s.targetAddr, 10*time.Second)
	if err != nil {
		log.Printf("[mitm %s] dial target failed: %v", s.id, err)
		return
	}
	defer target.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		for {
			n, err := client.Read(buf)
			if n > 0 {
				s.logTraffic("C->T", buf[:n])
				target.Write(buf[:n])
			}
			if err != nil {
				target.Close()
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		for {
			n, err := target.Read(buf)
			if n > 0 {
				s.logTraffic("T->C", buf[:n])
				client.Write(buf[:n])
			}
			if err != nil {
				client.Close()
				return
			}
		}
	}()

	wg.Wait()
}

func (s *mitmSession) logTraffic(direction string, data []byte) {
	ts := time.Now().Format("15:04:05.000")
	hexStr := hex.EncodeToString(data)

	// Spaced hex
	spaced := make([]string, len(data))
	for i, b := range data {
		spaced[i] = fmt.Sprintf("%02x", b)
	}

	// ASCII representation
	ascii := make([]byte, len(data))
	for i, b := range data {
		if b >= 0x20 && b < 0x7f {
			ascii[i] = b
		} else {
			ascii[i] = '.'
		}
	}

	line := fmt.Sprintf("%s %s (%d bytes): %s\n  ASCII: %s\n", ts, direction, len(data), strings.Join(spaced, " "), string(ascii))

	s.trafficMu.Lock()
	s.trafficLog.WriteString(line)
	s.trafficMu.Unlock()

	// Also write to file if configured
	if s.logPath != "" {
		f, err := os.OpenFile(s.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			fmt.Fprintf(f, "%s %s: %s\n  %s (%d bytes): %s\n  ASCII: %s\n", ts, direction, hexStr, direction, len(data), strings.Join(spaced, " "), string(ascii))
			f.Close()
		}
	}
}

func (a *Agent) handleMitmStop(env protocol.Envelope) protocol.Envelope {
	var params protocol.MitmStopParams
	if err := json.Unmarshal(env.Params, &params); err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}

	a.mitmMgr.mu.Lock()
	session, ok := a.mitmMgr.sessions[params.MitmID]
	if ok {
		delete(a.mitmMgr.sessions, params.MitmID)
	}
	a.mitmMgr.mu.Unlock()

	if !ok {
		return protocol.NewError(env.ID, protocol.ErrNotFound, "mitm session not found")
	}

	close(session.stop)
	session.listener.Close()

	return protocol.NewResult(env.ID, protocol.TypeMitmStopped, map[string]interface{}{
		"stopped": true,
		"mitm_id": params.MitmID,
	})
}

func (a *Agent) handleMitmTraffic(env protocol.Envelope) protocol.Envelope {
	var params protocol.MitmStopParams // reuse same struct (just needs MitmID)
	if err := json.Unmarshal(env.Params, &params); err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}

	a.mitmMgr.mu.Lock()
	session, ok := a.mitmMgr.sessions[params.MitmID]
	a.mitmMgr.mu.Unlock()

	if !ok {
		return protocol.NewError(env.ID, protocol.ErrNotFound, "mitm session not found")
	}

	session.trafficMu.Lock()
	traffic := session.trafficLog.String()
	session.trafficMu.Unlock()

	// Count entries
	entries := strings.Count(traffic, "C->T") + strings.Count(traffic, "T->C")

	return protocol.NewResult(env.ID, "mitm_traffic", map[string]interface{}{
		"mitm_id":  params.MitmID,
		"traffic":  traffic,
		"size":     len(traffic),
		"entries":  entries,
	})
}

func (a *Agent) closeAllMitm() {
	if a.mitmMgr == nil {
		return
	}
	a.mitmMgr.mu.Lock()
	defer a.mitmMgr.mu.Unlock()
	for id, session := range a.mitmMgr.sessions {
		close(session.stop)
		session.listener.Close()
		delete(a.mitmMgr.sessions, id)
	}
}