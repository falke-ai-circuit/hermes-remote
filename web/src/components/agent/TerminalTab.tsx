import { useState, useRef } from 'react'
import { api } from '../../api/client'

export function TerminalTab({ agentId }: { agentId: string }) {
  const [output, setOutput] = useState('')
  const [cmd, setCmd] = useState('')
  const [running, setRunning] = useState(false)
  const outputRef = useRef<HTMLDivElement>(null)

  const exec = async () => {
    if (!cmd.trim() || running) return
    setRunning(true)
    setOutput(prev => prev + `$ ${cmd}\n`)
    try {
      const res = await api.execCmd(agentId, cmd)
      const text = typeof res === 'string' ? res : JSON.stringify(res, null, 2)
      setOutput(prev => prev + text + '\n')
    } catch (e) {
      setOutput(prev => prev + `Error: ${(e as Error).message}\n`)
    } finally {
      setRunning(false)
      setCmd('')
      setTimeout(() => {
        if (outputRef.current) outputRef.current.scrollTop = outputRef.current.scrollHeight
      }, 50)
    }
  }

  return (
    <div className="card">
      <div className="terminal-input-row">
        <input
          type="text"
          value={cmd}
          onChange={e => setCmd(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && exec()}
          placeholder="Enter command…"
          disabled={running}
          autoFocus
        />
        <button className="btn btn-primary" onClick={exec} disabled={running || !cmd.trim()}>
          Run
        </button>
        <button className="btn" onClick={() => setOutput('')}>Clear</button>
      </div>
      <div className="terminal-output" ref={outputRef}>
        {output || 'Terminal ready. Type a command and press Enter.'}
      </div>
    </div>
  )
}