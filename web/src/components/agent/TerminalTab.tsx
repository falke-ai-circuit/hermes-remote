import { useState, useRef, useEffect, useCallback } from 'react'
import { api } from '../../api/client'

interface HistoryEntry {
  cmd: string
  output: string
}

export function TerminalTab({ agentId }: { agentId: string }) {
  const [history, setHistory] = useState<HistoryEntry[]>([])
  const [cmd, setCmd] = useState('')
  const [running, setRunning] = useState(false)
  const [cmdHistory, setCmdHistory] = useState<string[]>([])
  const [histIdx, setHistIdx] = useState(-1)
  const outputRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  const scrollDown = useCallback(() => {
    setTimeout(() => {
      if (outputRef.current) outputRef.current.scrollTop = outputRef.current.scrollHeight
    }, 50)
  }, [])

  const exec = async (command: string) => {
    if (!command.trim() || running) return
    setRunning(true)
    setCmdHistory(prev => [...prev, command])
    setHistIdx(-1)

    try {
      const res = await api.execCmd(agentId, command)
      const text = typeof res === 'string' ? res
        : (res as { output?: string })?.output
        || (res as { stdout?: string })?.stdout
        || JSON.stringify(res, null, 2)
      setHistory(prev => [...prev, { cmd: command, output: text }])
    } catch (e) {
      setHistory(prev => [...prev, { cmd: command, output: `Error: ${(e as Error).message}` }])
    } finally {
      setRunning(false)
      setCmd('')
      scrollDown()
    }
  }

  const handleKey = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      exec(cmd)
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      if (cmdHistory.length === 0) return
      const newIdx = histIdx === -1 ? cmdHistory.length - 1 : Math.max(0, histIdx - 1)
      setHistIdx(newIdx)
      setCmd(cmdHistory[newIdx])
    } else if (e.key === 'ArrowDown') {
      e.preventDefault()
      if (histIdx === -1) return
      const newIdx = histIdx + 1
      if (newIdx >= cmdHistory.length) {
        setHistIdx(-1)
        setCmd('')
      } else {
        setHistIdx(newIdx)
        setCmd(cmdHistory[newIdx])
      }
    } else if (e.key === 'l' && e.ctrlKey) {
      e.preventDefault()
      setHistory([])
    }
  }

  useEffect(() => { inputRef.current?.focus() }, [])

  return (
    <div>
      <div className="toolbar">
        <button className="btn btn-sm" onClick={() => setHistory([])}>Clear</button>
        <span className="dim" style={{ fontSize: 11, marginLeft: 8 }}>
          ↑↓ history · Ctrl+L clear
        </span>
      </div>
      <div className="terminal-output" ref={outputRef} onClick={() => inputRef.current?.focus()}>
        {history.length === 0 ? (
          <span className="dim">PROBE Terminal ready. Type a command and press Enter.</span>
        ) : (
          history.map((h, i) => (
            <div key={i}>
              <span style={{ color: '#888' }}>$ </span>
              <span style={{ color: '#fff' }}>{h.cmd}</span>
              {'\n'}
              {h.output}
              {'\n'}
            </div>
          ))
        )}
        {running && <span className="dim">Executing…</span>}
      </div>
      <div className="terminal-input-row">
        <span style={{ color: 'var(--probe-green)', fontFamily: 'monospace', padding: '8px 0', fontSize: 13 }}>$</span>
        <input
          ref={inputRef}
          type="text"
          value={cmd}
          onChange={e => setCmd(e.target.value)}
          onKeyDown={handleKey}
          placeholder="Enter command…"
          disabled={running}
          autoFocus
          style={{ flex: 1 }}
        />
        <button className="btn btn-primary btn-sm" onClick={() => exec(cmd)} disabled={running || !cmd.trim()}>
          Run
        </button>
      </div>
    </div>
  )
}