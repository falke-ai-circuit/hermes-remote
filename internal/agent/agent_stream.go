package agent

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/falke-ai-circuit/probe/internal/protocol"
	"github.com/gorilla/websocket"
)

// activeStream tracks a running screen stream.
type activeStream struct {
	streamID string
	conn     *websocket.Conn
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// streamManager manages active screen streams on the agent.
type streamManager struct {
	mu       sync.Mutex
	streams  map[string]*activeStream
	nextSeq  int
}

func newStreamManager() *streamManager {
	return &streamManager{
		streams: make(map[string]*activeStream),
	}
}

// handleStreamBegin starts screen streaming on the agent.
// It calls the platform's ScreenStreamStart, which captures frames at the
// specified FPS and sends them back via WebSocket as stream_data messages.
func (a *Agent) handleStreamBegin(env protocol.Envelope) protocol.Envelope {
	params, err := protocol.ParseCommand[protocol.ScreenStreamParams](env)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrInvalidParams, err.Error())
	}

	if params.FPS <= 0 {
		params.FPS = 10
	}
	if params.Quality <= 0 {
		params.Quality = 80
	}

	// Start the platform-specific screen stream
	result, err := a.plat.ScreenStreamStart(params.Display, params.FPS, params.Quality)
	if err != nil {
		return protocol.NewError(env.ID, protocol.ErrPlatformNotSupported, err.Error())
	}

	// If the platform returns a stream ID, use it; otherwise generate one
	if result.StreamID == "" {
		result.StreamID = fmt.Sprintf("stream-%d", time.Now().UnixNano())
	}

	// Register the stream and start sending frames
	a.mu.Lock()
	conn := a.conn
	a.mu.Unlock()

	if conn != nil {
		stream := &activeStream{
			streamID: result.StreamID,
			conn:     conn,
			stopCh:   make(chan struct{}),
			doneCh:   make(chan struct{}),
		}
		if a.streamMgr == nil {
			a.streamMgr = newStreamManager()
		}
		a.streamMgr.mu.Lock()
		a.streamMgr.streams[result.StreamID] = stream
		a.streamMgr.mu.Unlock()

		// Start frame capture goroutine
		go a.streamFrames(stream, params)
	}

	return protocol.NewResult(env.ID, protocol.TypeStreamBeginResult, result)
}

// handleStreamEnd stops screen streaming on the agent.
func (a *Agent) handleStreamEnd(env protocol.Envelope) protocol.Envelope {
	params, err := protocol.ParseCommand[protocol.ScreenStreamStopParams](env)
	if err != nil {
		// Allow empty params — stop all streams
		if a.streamMgr != nil {
			a.streamMgr.mu.Lock()
			for id, stream := range a.streamMgr.streams {
				close(stream.stopCh)
				delete(a.streamMgr.streams, id)
			}
			a.streamMgr.mu.Unlock()
		}
		_ = a.plat.ScreenStreamStop("")
		return protocol.NewResult(env.ID, protocol.TypeStreamEndResult, map[string]interface{}{
			"stopped": true,
		})
	}

	// Stop the platform stream
	_ = a.plat.ScreenStreamStop(params.StreamID)

	// Stop the frame sender
	if a.streamMgr != nil {
		a.streamMgr.mu.Lock()
		if stream, ok := a.streamMgr.streams[params.StreamID]; ok {
			close(stream.stopCh)
			delete(a.streamMgr.streams, params.StreamID)
		}
		a.streamMgr.mu.Unlock()
	}

	return protocol.NewResult(env.ID, protocol.TypeStreamEndResult, map[string]interface{}{
		"stopped":   true,
		"stream_id": params.StreamID,
	})
}

// streamFrames is the platform-agnostic frame sender. It calls the platform's
// CaptureDisplay at the specified FPS and sends each frame as a stream_data
// message. The platform's ScreenStreamStart may alternatively handle its own
// frame capture (e.g. Windows using System.Drawing), in which case this
// goroutine acts as a coordinator that waits for the stop signal.
func (a *Agent) streamFrames(stream *activeStream, params protocol.ScreenStreamParams) {
	defer close(stream.doneCh)

	interval := time.Second / time.Duration(params.FPS)
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	seqNum := 0

	for {
		select {
		case <-stream.stopCh:
			log.Printf("[stream] %s stopped", stream.streamID)
			return
		case <-ticker.C:
			// Capture a frame
			result, err := a.plat.CaptureDisplay(params.Display, params.Quality)
			if err != nil {
				log.Printf("[stream] capture error: %v", err)
				continue
			}

			seqNum++
			frameData := protocol.StreamDataParams{
				StreamID:  stream.streamID,
				Frame:     result.Data,
				Width:     result.Width,
				Height:    result.Height,
				SeqNum:    seqNum,
				Timestamp: time.Now().UnixMilli(),
			}

			env := protocol.Envelope{
				ID:     fmt.Sprintf("stream-%s-%d", stream.streamID, seqNum),
				Type:   protocol.TypeStreamData,
				Result: mustMarshal(frameData),
			}

			a.mu.Lock()
			conn := a.conn
			a.mu.Unlock()

			if conn == nil {
				log.Printf("[stream] connection lost, stopping")
				return
			}

			if err := a.writeMessage(conn, env); err != nil {
				log.Printf("[stream] write error: %v", err)
				return
			}
		}
	}
}

// closeAllStreams stops all active screen streams. Called on disconnect.
func (a *Agent) closeAllStreams() {
	if a.streamMgr == nil {
		return
	}
	a.streamMgr.mu.Lock()
	defer a.streamMgr.mu.Unlock()
	for id, stream := range a.streamMgr.streams {
		close(stream.stopCh)
		delete(a.streamMgr.streams, id)
	}
}