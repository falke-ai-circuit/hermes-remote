import { useState } from 'react'
import { api } from '../../api/client'

export function DebugTab({ agentId }: { agentId: string }) {
  const [pid, setPid] = useState('')
  const [addr, setAddr] = useState('')
  const [size, setSize] = useState('256')
  const [output, setOutput] = useState('')
  const [error, setError] = useState('')
  const [attached, setAttached] = useState(false)

  const attach = async () => {
    setError('')
    try {
      await api.debugAttach(agentId, parseInt(pid))
      setAttached(true)
      setOutput('Attached to PID ' + pid)
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const detach = async () => {
    setError('')
    try {
      await api.debugDetach(agentId)
      setAttached(false)
      setOutput('Detached')
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const readMem = async () => {
    setError('')
    try {
      const res = await api.debugReadMem(agentId, addr, parseInt(size))
      setOutput(JSON.stringify(res, null, 2))
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const getModules = async () => {
    setError('')
    try {
      const res = await api.debugModules(agentId)
      setOutput(JSON.stringify(res, null, 2))
    } catch (e) {
      setError((e as Error).message)
    }
  }

  return (
    <div className="card">
      <div className="card-title">Debug</div>
      {error && <div className="error-msg">{error}</div>}

      <div className="form-row mb-16">
        <div className="form-group">
          <label>Target PID</label>
          <input type="number" value={pid} onChange={e => setPid(e.target.value)} placeholder="1234" />
        </div>
        <div className="form-group">
          <label>Actions</label>
          <div className="flex gap-8">
            <button className="btn btn-primary btn-sm" onClick={attach} disabled={attached}>Attach</button>
            <button className="btn btn-danger btn-sm" onClick={detach} disabled={!attached}>Detach</button>
            <button className="btn btn-sm" onClick={getModules} disabled={!attached}>Modules</button>
          </div>
        </div>
      </div>

      <div className="form-row mb-16">
        <div className="form-group">
          <label>Memory Address (hex)</label>
          <input type="text" value={addr} onChange={e => setAddr(e.target.value)} placeholder="0x400000" />
        </div>
        <div className="form-group">
          <label>Size (bytes)</label>
          <input type="number" value={size} onChange={e => setSize(e.target.value)} placeholder="256" />
        </div>
        <div className="form-group">
          <label>&nbsp;</label>
          <button className="btn btn-sm" onClick={readMem} disabled={!attached}>Read Memory</button>
        </div>
      </div>

      {output && <div className="terminal-output mt-16">{output}</div>}
    </div>
  )
}