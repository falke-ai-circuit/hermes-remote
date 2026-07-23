import { useState } from 'react'
import { api } from '../../api/client'

export function TunnelsTab({ agentId }: { agentId: string }) {
  const [port, setPort] = useState('')
  const [target, setTarget] = useState('')
  const [tunnelId, setTunnelId] = useState('')
  const [output, setOutput] = useState('')
  const [error, setError] = useState('')

  const openTunnel = async () => {
    setError('')
    setOutput('')
    try {
      const res = await api.tunnelOpen(agentId, parseInt(port), target)
      const data = res as { tunnel_id?: string; [k: string]: unknown }
      if (data.tunnel_id) setTunnelId(data.tunnel_id)
      setOutput(JSON.stringify(res, null, 2))
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const closeTunnel = async () => {
    setError('')
    try {
      const res = await api.tunnelClose(agentId, tunnelId)
      setOutput(JSON.stringify(res, null, 2))
      setTunnelId('')
    } catch (e) {
      setError((e as Error).message)
    }
  }

  return (
    <div className="card">
      <div className="card-title">TCP Tunnels</div>
      {error && <div className="error-msg">{error}</div>}
      <div className="form-row mb-16">
        <div className="form-group">
          <label>Local Port</label>
          <input type="number" value={port} onChange={e => setPort(e.target.value)} placeholder="8080" />
        </div>
        <div className="form-group">
          <label>Target Address</label>
          <input type="text" value={target} onChange={e => setTarget(e.target.value)} placeholder="10.0.0.5:80" />
        </div>
      </div>
      <div className="flex gap-8">
        <button className="btn btn-primary btn-sm" onClick={openTunnel}>Open Tunnel</button>
        {tunnelId && (
          <button className="btn btn-danger btn-sm" onClick={closeTunnel}>Close Tunnel ({tunnelId.slice(0, 8)})</button>
        )}
      </div>
      {output && <div className="terminal-output mt-16">{output}</div>}
    </div>
  )
}