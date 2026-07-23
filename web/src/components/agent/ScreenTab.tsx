import { useState, useRef, useEffect } from 'react'
import { api } from '../../api/client'
import { Camera, Video, Square, Monitor } from 'lucide-react'

export function ScreenTab({ agentId }: { agentId: string }) {
  const [screenshot, setScreenshot] = useState('')
  const [error, setError] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [streamMode, setStreamMode] = useState<'screenshot' | 'stream'>('screenshot')
  const streamRef = useRef<ReturnType<typeof setInterval>>()

  const capture = async () => {
    setError('')
    try {
      const res = await api.capture(agentId)
      if (typeof res === 'string') { setScreenshot(res.startsWith('data:') ? res : `data:image/jpeg;base64,${res}`) }
      else {
        const d = res as { data?: string; image?: string; base64?: string; url?: string; screenshot?: string; format?: string }
        const fmt = d.format || 'jpeg'
        if (d.data) setScreenshot(`data:image/${fmt};base64,${d.data}`)
        else if (d.image) setScreenshot(d.image.startsWith('data:') ? d.image : `data:image/${fmt};base64,${d.image}`)
        else if (d.base64) setScreenshot(`data:image/${fmt};base64,${d.base64}`)
        else if (d.url) setScreenshot(d.url)
        else if (d.screenshot) setScreenshot(d.screenshot.startsWith('data:') ? d.screenshot : `data:image/${fmt};base64,${d.screenshot}`)
      }
    } catch (e) { setError((e as Error).message) }
  }

  const startStream = async () => {
    setStreaming(true)
    const tick = async () => { try { const res = await api.capture(agentId); if (typeof res === 'string') { setScreenshot(res.startsWith('data:') ? res : `data:image/jpeg;base64,${res}`) } else { const d = res as { data?: string; format?: string }; if (d.data) setScreenshot(`data:image/${d.format || 'jpeg'};base64,${d.data}`) } } catch (e) { setError((e as Error).message); stopStream() } }
    tick(); streamRef.current = setInterval(tick, 2000)
  }
  const stopStream = () => { setStreaming(false); if (streamRef.current) { clearInterval(streamRef.current); streamRef.current = undefined } }

  useEffect(() => { return () => { if (streamRef.current) clearInterval(streamRef.current) } }, [])

  return (
    <div>
      {error && <div className="error-msg">{error}</div>}
      <div className="toolbar">
        <button className={`btn btn-sm ${streamMode === 'screenshot' ? 'btn-primary' : ''}`} onClick={() => { stopStream(); setStreamMode('screenshot') }}><Camera size={14} /> Screenshot</button>
        <button className={`btn btn-sm ${streamMode === 'stream' ? 'btn-primary' : ''}`} onClick={() => { stopStream(); setStreamMode('stream') }}><Video size={14} /> Stream</button>
        <span className="toolbar-spacer" />
        {streamMode === 'screenshot' ? (
          <button className="btn btn-primary btn-sm" onClick={capture} disabled={streaming}><Camera size={14} /> Capture Screen</button>
        ) : (
          <>
            <span className={`status-dot ${streaming ? 'active' : 'inactive'}`} />
            {streaming ? <button className="btn btn-danger btn-sm" onClick={stopStream}><Square size={14} /> Stop Stream</button> : <button className="btn btn-primary btn-sm" onClick={startStream}><Video size={14} /> Start Stream</button>}
            <span className="dim" style={{ fontSize: 11 }}>2s interval</span>
          </>
        )}
      </div>
      {screenshot ? (
        <div className="screen-display">
          <img src={screenshot} alt="Screen capture" style={{ maxWidth: '100%', borderRadius: 3 }} />
          <div className="dim" style={{ fontSize: 11, marginTop: 6, textAlign: 'center' }}>{streaming ? 'Streaming…' : `Captured at ${new Date().toLocaleString()}`}</div>
        </div>
      ) : (
        <div className="screen-stream"><div className="empty-state"><Monitor size={32} style={{ opacity: 0.3, marginBottom: 8 }} />{streamMode === 'screenshot' ? 'No screenshot captured. Click "Capture Screen" to grab a frame.' : 'Stream not started. Click "Start Stream" for live screen capture.'}</div></div>
      )}
    </div>
  )
}