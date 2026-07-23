import { useState } from 'react'
import { api } from '../../api/client'

export function MITMTab({ agentId }: { agentId: string }) {
  const [target, setTarget] = useState('')
  const [port, setPort] = useState('')
  const [traffic, setTraffic] = useState('')
  const [error, setError] = useState('')
  const [active, setActive] = useState(false)

  const start = async () => {
    setError('')
    try {
      await api.mitmStart(agentId, target, parseInt(port))
      setActive(true)
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const stop = async () => {
    setError('')
    try {
      await api.mitmStop(agentId)
      setActive(false)
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const getTraffic = async () => {
    setError('')
    try {
      const res = await api.mitmTraffic(agentId)
      setTraffic(JSON.stringify(res, null, 2))
    } catch (e) {
      setError((e as Error).message)
    }
  }

  return (
    <div className="card">
      <div className="card-title">MITM Proxy</div>
      {error && <div className="error-msg">{error}</div>}
      <div className="form-row mb-16">
        <div className="form-group">
          <label>Target Address</label>
          <input type="text" value={target} onChange={e => setTarget(e.target.value)} placeholder="10.0.0.5" />
        </div>
        <div className="form-group">
          <label>Target Port</label>
          <input type="number" value={port} onChange={e => setPort(e.target.value)} placeholder="80" />
        </div>
      </div>
      <div className="flex gap-8 mb-16">
        <button className="btn btn-primary btn-sm" onClick={start} disabled={active}>Start MITM</button>
        <button className="btn btn-danger btn-sm" onClick={stop} disabled={!active}>Stop MITM</button>
        <button className="btn btn-sm" onClick={getTraffic}>Get Traffic</button>
      </div>
      {traffic && <div className="terminal-output">{traffic}</div>}
    </div>
  )
}