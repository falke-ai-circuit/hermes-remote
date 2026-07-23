import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import type { AgentRecord } from '../api/types'
import { StatusBadge } from '../components/StatusBadge'

export default function Agents() {
  const [agents, setAgents] = useState<AgentRecord[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const load = async () => {
      try {
        const a = await api.listAgents()
        setAgents(a || [])
        setError('')
      } catch (e) {
        setError((e as Error).message)
      } finally {
        setLoading(false)
      }
    }
    load()
    const interval = setInterval(load, 5000)
    return () => clearInterval(interval)
  }, [])

  return (
    <div>
      <div className="page-header">
        <h1>Agents</h1>
        <p>Connected remote agents and their status</p>
      </div>

      {error && <div className="error-msg">{error}</div>}

      {loading ? (
        <div className="loading">Loading agents…</div>
      ) : agents.length === 0 ? (
        <div className="card">
          <div className="empty-state">No agents registered</div>
        </div>
      ) : (
        <div className="card">
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Agent ID</th>
                  <th>OS / Arch</th>
                  <th>Mode</th>
                  <th>Status</th>
                  <th>Health</th>
                  <th>Capabilities</th>
                  <th>Connected</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {agents.map(a => (
                  <tr key={a.agent_id}>
                    <td>
                      <Link to={`/agents/${a.agent_id}`}>{a.name || a.agent_id}</Link>
                    </td>
                    <td className="mono dim">{a.agent_id.slice(0, 16)}…</td>
                    <td className="mono">{a.os}/{a.arch}</td>
                    <td>{a.mode}</td>
                    <td><StatusBadge status={a.status} /></td>
                    <td>
                      <div className="flex gap-8" style={{ alignItems: 'center' }}>
                        <div style={{ width: '60px', height: '6px', background: 'var(--border)', borderRadius: '3px', overflow: 'hidden' }}>
                          <div style={{
                            width: `${a.health_score * 100}%`,
                            height: '100%',
                            background: a.health_score > 0.7 ? 'var(--valmet-green)' : a.health_score > 0.4 ? 'var(--yellow)' : 'var(--red)',
                          }} />
                        </div>
                        <span className="mono dim">{(a.health_score * 100).toFixed(0)}%</span>
                      </div>
                    </td>
                    <td className="mono dim">{(a.capabilities || []).join(', ')}</td>
                    <td className="dim">{a.connected_at ? new Date(a.connected_at).toLocaleString() : '—'}</td>
                    <td>
                      <Link to={`/agents/${a.agent_id}`} className="btn btn-sm">Manage</Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}