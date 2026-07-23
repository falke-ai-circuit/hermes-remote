import { useState } from 'react'
import { api } from '../../api/client'
import { ArrowLeftRight, X, Plus } from 'lucide-react'

interface Tunnel { id: string; local_port: number; target_address: string; status: string; created_at?: string }

export function TunnelsTab({ agentId }: { agentId: string }) {
  const [port, setPort] = useState(''); const [target, setTarget] = useState('')
  const [tunnels, setTunnels] = useState<Tunnel[]>([])
  const [error, setError] = useState(''); const [output, setOutput] = useState('')

  const openTunnel = async () => {
    setError(''); setOutput('')
    try { const res = await api.tunnelOpen(agentId, parseInt(port), target); const d = res as { tunnel_id?: string }; setTunnels(prev => [...prev, { id: d.tunnel_id || `tun-${Date.now()}`, local_port: parseInt(port), target_address: target, status: 'active', created_at: new Date().toISOString() }]); setOutput(`Tunnel opened: localhost:${port} → ${target}`); setPort(''); setTarget('') }
    catch (e) { setError((e as Error).message) }
  }
  const closeTunnel = async (id: string) => { setError(''); try { await api.tunnelClose(agentId, id); setTunnels(prev => prev.map(t => t.id === id ? { ...t, status: 'closed' } : t)); setOutput(`Tunnel closed`) } catch (e) { setError((e as Error).message) } }
  const removeTunnel = (id: string) => setTunnels(prev => prev.filter(t => t.id !== id))

  return (
    <div>
      {error && <div className="error-msg">{error}</div>}
      {output && <div className="success-msg">{output}</div>}
      <div className="card">
        <div className="card-title"><ArrowLeftRight size={12} style={{ display: 'inline' }} /> Open New Tunnel</div>
        <div className="form-row">
          <div className="form-group"><label>Local Port</label><input type="number" value={port} onChange={e => setPort(e.target.value)} placeholder="8080" /></div>
          <div className="form-group"><label>Target Address</label><input type="text" value={target} onChange={e => setTarget(e.target.value)} placeholder="10.0.0.5:80" /></div>
          <div className="form-group" style={{ flex: 0 }}><label>&nbsp;</label><button className="btn btn-primary btn-sm" onClick={openTunnel} disabled={!port || !target}><Plus size={14} /> Open</button></div>
        </div>
      </div>
      <div className="card">
        <div className="card-title">Active Tunnels ({tunnels.filter(t => t.status === 'active').length})</div>
        {tunnels.length === 0 ? <div className="empty-state">No tunnels configured</div> :
          tunnels.map(t => (
            <div key={t.id} className={`tunnel-card ${t.status === 'active' ? 'tunnel-active' : 'tunnel-closed'}`}>
              <span className={`status-dot ${t.status === 'active' ? 'active' : 'inactive'}`} />
              <div style={{ flex: 1 }}>
                <div className="mono" style={{ fontSize: 14 }}>localhost:{t.local_port} <span className="dim">→</span> {t.target_address}</div>
                <div className="dim" style={{ fontSize: 11 }}>ID: {t.id.slice(0, 16)} · {t.status === 'active' ? 'Active' : 'Closed'}{t.created_at && ` · ${new Date(t.created_at).toLocaleString()}`}</div>
              </div>
              {t.status === 'active' ? <button className="btn btn-danger btn-sm" onClick={() => closeTunnel(t.id)}><X size={14} /> Close</button> : <button className="btn btn-sm" onClick={() => removeTunnel(t.id)}>Remove</button>}
            </div>
          ))}
      </div>
    </div>
  )
}