import { useState, useRef, useEffect } from 'react'
import { api } from '../../api/client'

export function ScreenTab({ agentId }: { agentId: string }) {
  const [screenshot, setScreenshot] = useState<string>('')
  const [error, setError] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [streamMode, setStreamMode] = useState<'screenshot' | 'stream'>('screenshot')
  const streamRef = useRef<ReturnType<typeof setInterval>>()

  const capture = async () => {
    setError('')
    try {
      const res = await api.capture(agentId)
      // Response might contain base64 image or a URL
      if (typeof res === 'string') {
        setScreenshot(res.startsWith('data:') ? res : `data:image/jpeg;base64,${res}`)
      } else {
        const data = res as { data?: string; image?: string; base64?: string; url?: string; screenshot?: string; format?: string }
        const fmt = data.format || 'jpeg'
        if (data.data) setScreenshot(`data:image/${fmt};base64,${data.data}`)
        else if (data.image) setScreenshot(data.image.startsWith('data:') ? data.image : `data:image/${fmt};base64,${data.image}`)
        else if (data.base64) setScreenshot(`data:image/${fmt};base64,${data.base64}`)
        else if (data.url) setScreenshot(data.url)
        else if (data.screenshot) setScreenshot(data.screenshot.startsWith('data:') ? data.screenshot : `data:image/${fmt};base64,${data.screenshot}`)
        else setScreenshot('')
      }
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const startStream = async () => {
    setStreaming(true)
    const tick = async () => {
      try {
        const res = await api.capture(agentId)
        if (typeof res === 'string') {
          setScreenshot(res.startsWith('data:') ? res : `data:image/jpeg;base64,${res}`)
        } else {
          const data = res as { data?: string; image?: string; base64?: string; url?: string; screenshot?: string; format?: string }
          const fmt = data.format || 'jpeg'
          if (data.data) setScreenshot(`data:image/${fmt};base64,${data.data}`)
          else if (data.image) setScreenshot(data.image.startsWith('data:') ? data.image : `data:image/${fmt};base64,${data.image}`)
          else if (data.base64) setScreenshot(`data:image/${fmt};base64,${data.base64}`)
          else if (data.url) setScreenshot(data.url)
          else if (data.screenshot) setScreenshot(data.screenshot.startsWith('data:') ? data.screenshot : `data:image/${fmt};base64,${data.screenshot}`)
        }
      } catch (e) {
        setError((e as Error).message)
        stopStream()
      }
    }
    tick()
    streamRef.current = setInterval(tick, 2000) // 0.5 fps for bandwidth
  }

  const stopStream = () => {
    setStreaming(false)
    if (streamRef.current) {
      clearInterval(streamRef.current)
      streamRef.current = undefined
    }
  }

  useEffect(() => {
    return () => { if (streamRef.current) clearInterval(streamRef.current) }
  }, [])

  return (
    <div>
      {error && <div className="error-msg">{error}</div>}

      <div className="toolbar">
        <button
          className={`btn btn-sm ${streamMode === 'screenshot' ? 'btn-primary' : ''}`}
          onClick={() => { stopStream(); setStreamMode('screenshot') }}
        >
          📸 Screenshot
        </button>
        <button
          className={`btn btn-sm ${streamMode === 'stream' ? 'btn-primary' : ''}`}
          onClick={() => { stopStream(); setStreamMode('stream') }}
        >
          ⛶ Stream
        </button>
        <span className="toolbar-spacer" />
        {streamMode === 'screenshot' ? (
          <button className="btn btn-primary btn-sm" onClick={capture} disabled={streaming}>
            Capture Screen
          </button>
        ) : (
          <>
            <span className={`status-dot ${streaming ? 'active' : 'inactive'}`} />
            {streaming ? (
              <button className="btn btn-danger btn-sm" onClick={stopStream}>Stop Stream</button>
            ) : (
              <button className="btn btn-primary btn-sm" onClick={startStream}>Start Stream</button>
            )}
            <span className="dim" style={{ fontSize: 11 }}>2s interval</span>
          </>
        )}
      </div>

      {screenshot ? (
        <div className="screen-display">
          <img src={screenshot} alt="Screen capture" style={{ maxWidth: '100%', borderRadius: 4 }} />
          <div className="dim" style={{ fontSize: 11, marginTop: 6, textAlign: 'center' }}>
            {streaming ? 'Streaming…' : `Captured at ${new Date().toLocaleString()}`}
          </div>
        </div>
      ) : (
        <div className="screen-stream">
          <div className="empty-state">
            {streamMode === 'screenshot'
              ? 'No screenshot captured. Click "Capture Screen" to grab a frame.'
              : 'Stream not started. Click "Start Stream" for live screen capture (2s refresh).'}
          </div>
        </div>
      )}
    </div>
  )
}