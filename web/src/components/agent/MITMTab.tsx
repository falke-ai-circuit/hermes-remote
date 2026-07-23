import { useState, useEffect, useRef } from 'react'
import { api } from '../../api/client'
import { Network, Play, Square, Activity, Trash2, Pencil, Check } from 'lucide-react'

interface MITMSession { id: string; target: string; port: number; active: boolean }

export function MITMTab({ agentId }: { agentId: string }) {
  const [target, setTarget] = useState(''); const [port, setPort] = useState('')
  const [sessions, setSessions] = useState<MITMSession[]>([])
  const [error, setError] = useState('')
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editTarget, setEditTarget] = useState(''); const [editPort, setEditPort] = useState('')
  const [activeTraffic, setActiveTraffic] = useState<string | null>(null)
  const [trafficContent, setTrafficContent] = useState('')
  const trafficRef = useRef<ReturnType<typeof setInterval>>()

  const start = async () => { setError(''); try { await api.mitmStart(agentId, target, parseInt(port)); setSessions(prev => [...prev, { id: `mitm-${Date.now()}`, target, port: parseInt(port), active: true }]); setTarget(''); setPort('') } catch (e) { setError((e as Error).message) } }
  const stop = async (id: string) => { setError(''); try { await api.mitmStop(agentId); setSessions(prev => prev.map(s => s.id === id ? { ...s, active: false } : s)); if (activeTraffic === id) { setActiveTraffic(null); if (trafficRef.current) clearInterval(trafficRef.current) } } catch (e) { setError((e as Error).message) } }
  const removeSession = (id: string) => { setSessions(prev => prev.filter(s => s.id !== id)); if (activeTraffic === id) { setActiveTraffic(null); if (trafficRef.current) clearInterval(trafficRef.current) } }
  const editSession = (s: MITMSession) => { setEditingId(s.id); setEditTarget(s.target); setEditPort(String(s.port)) }
  const saveEdit = (id: string) => { setSessions(prev => prev.map(s => s.id === id ? { ...s, target: editTarget, port: parseInt(editPort) } : s)); setEditingId(null) }

  const viewTraffic = async (id: string) => {
    if (activeTraffic === id) { setActiveTraffic(null); if (trafficRef.current) clearInterval(trafficRef.current); return }
    setActiveTraffic(id)
    const fetchTraffic = async () => { try { const res = await api.mitmTraffic(agentId); setTrafficContent(typeof res === 'string' ? res : JSON.stringify(res, null, 2)) } catch (e) { setTrafficContent(`Error: ${(e as Error).message}`) } }
    fetchTraffic(); trafficRef.current = setInterval(fetchTraffic, 2000)
  }

  useEffect(() => { return () => { if (trafficRef.current) clearInterval(trafficRef.current) } }, [])

  return (
    <div>
      {error && <div className="error-msg">{error}</div>}
      <div className="card">
        <div className="card-title"><Network size={12} style={{ display: 'inline' }} /> Start New MITM Session</div>
        <div className="form-row">
          <div className="form-group"><label>Target Address</label><input type="text" value={target} onChange={e => setTarget(e.target.value)} placeholder="10.0.0.5" /></div>
          <div className="form-group"><label>Target Port</label><input type="number" value={port} onChange={e => setPort(e.target.value)} placeholder="80" /></div>
          <div className="form-group" style={{ flex: 0 }}><label>&nbsp;</label><button className="btn btn-primary btn-sm" onClick={start} disabled={!target || !port}><Play size={14} /> Start</button></div>
        </div>
      </div>
      <div className="card">
        <div className="card-title">MITM Sessions ({sessions.filter(s => s.active).length} active)</div>
        {sessions.length === 0 ? <div className="empty-state">No MITM sessions</div> :
          sessions.map(s => (
            <div key={s.id} className="mitm-session" style={{ borderLeftColor: s.active ? 'var(--green)' : 'var(--text-dim)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 8 }}>
                <span className={`status-dot ${s.active ? 'active' : 'inactive'}`} />
                {editingId === s.id ? (
                  <div className="flex gap-8" style={{ flex: 1 }}>
                    <input type="text" value={editTarget} onChange={e => setEditTarget(e.target.value)} style={{ padding: '4px 8px', border: '1px solid var(--border)', borderRadius: 4, background: 'var(--bg-input)', color: 'var(--text)', fontSize: 13 }} />
                    <input type="number" value={editPort} onChange={e => setEditPort(e.target.value)} style={{ width: 80, padding: '4px 8px', border: '1px solid var(--border)', borderRadius: 4, background: 'var(--bg-input)', color: 'var(--text)', fontSize: 13 }} />
                    <button className="btn btn-sm btn-primary" onClick={() => saveEdit(s.id)}><Check size={14} /></button>
                    <button className="btn btn-sm" onClick={() => setEditingId(null)}>Cancel</button>
                  </div>
                ) : (
                  <>
                    <span className="mono" style={{ fontSize: 14 }}>{s.target}:{s.port}</span>
                    <span style={{ flex: 1 }} />
                    {s.active && <button className={`btn btn-sm ${activeTraffic === s.id ? 'btn-primary' : ''}`} onClick={() => viewTraffic(s.id)}><Activity size={14} /> {activeTraffic === s.id ? 'Stop' : 'Traffic'}</button>}
                    <button className="btn btn-sm" onClick={() => editSession(s)}><Pencil size={14} /> Edit</button>
                    {s.active ? <button className="btn btn-danger btn-sm" onClick={() => stop(s.id)}><Square size={14} /> Stop</button> : <button className="btn btn-danger btn-sm" onClick={() => removeSession(s.id)}><Trash2 size={14} /> Delete</button>}
                  </>
                )}
              </div>
              <div className="dim" style={{ fontSize: 11 }}>ID: {s.id.slice(0, 16)} · {s.active ? 'Active' : 'Stopped'}</div>
            </div>
          ))}
      </div>
      {activeTraffic && (
        <div className="card">
          <div className="card-title"><Activity size={12} style={{ display: 'inline' }} /> Live Traffic — {sessions.find(s => s.id === activeTraffic)?.target}:{sessions.find(s => s.id === activeTraffic)?.port}</div>
          <div className="traffic-viewer">{trafficContent || 'Waiting for traffic…'}</div>
        </div>
      )}
    </div>
  )
}