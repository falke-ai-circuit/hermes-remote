import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api } from '../api/client'
import type { AgentRecord, AuditEntry } from '../api/types'
import { StatusBadge } from '../components/StatusBadge'
import { TerminalTab } from '../components/agent/TerminalTab'
import { FilesTab } from '../components/agent/FilesTab'
import { ProcessesTab } from '../components/agent/ProcessesTab'
import { TunnelsTab } from '../components/agent/TunnelsTab'
import { MITMTab } from '../components/agent/MITMTab'
import { DebugTab } from '../components/agent/DebugTab'
import { ScreenTab } from '../components/agent/ScreenTab'

const tabs = ['Terminal', 'Files', 'Processes', 'Tunnels', 'MITM', 'Debug', 'Screen', 'Audit'] as const
type TabName = typeof tabs[number]

export default function AgentDetail() {
  const { id } = useParams<{ id: string }>()
  const [agent, setAgent] = useState<AgentRecord | null>(null)
  const [audit, setAudit] = useState<AuditEntry[]>([])
  const [activeTab, setActiveTab] = useState<TabName>('Terminal')
  const [error, setError] = useState('')

  useEffect(() => {
    if (!id) return
    const load = async () => {
      try {
        const [a, au] = await Promise.all([
          api.getAgent(id),
          api.getAgentAudit(id).catch(() => []),
        ])
        setAgent(a)
        setAudit(au || [])
        setError('')
      } catch (e) {
        setError((e as Error).message)
      }
    }
    load()
    const interval = setInterval(load, 5000)
    return () => clearInterval(interval)
  }, [id])

  if (error && !agent) return <div className="error-msg">{error}</div>
  if (!agent) return <div className="loading">Loading agent…</div>

  return (
    <div>
      <div className="page-header">
        <div className="flex gap-8" style={{ alignItems: 'center' }}>
          <Link to="/agents" className="btn btn-sm">← Agents</Link>
          <h1 style={{ marginLeft: 8 }}>{agent.name || agent.agent_id}</h1>
          <StatusBadge status={agent.status} />
        </div>
        <p className="mono dim">{agent.agent_id}</p>
      </div>

      <div className="card">
        <div className="card-title">Connection Info</div>
        <div className="table-container">
          <table>
            <tbody>
              <tr><td style={{ width: '150px' }} className="muted">Name</td><td>{agent.name}</td></tr>
              <tr><td className="muted">Version</td><td>{agent.version}</td></tr>
              <tr><td className="muted">OS / Arch</td><td className="mono">{agent.os} / {agent.arch}</td></tr>
              <tr><td className="muted">Mode</td><td>{agent.mode}</td></tr>
              <tr><td className="muted">Status</td><td><StatusBadge status={agent.status} /></td></tr>
              <tr><td className="muted">Health Score</td><td className="mono">{(agent.health_score * 100).toFixed(1)}%</td></tr>
              <tr><td className="muted">Connected At</td><td className="dim">{agent.connected_at ? new Date(agent.connected_at).toLocaleString() : '—'}</td></tr>
              <tr><td className="muted">Last Heartbeat</td><td className="dim">{agent.last_heartbeat ? new Date(agent.last_heartbeat).toLocaleString() : '—'}</td></tr>
              <tr><td className="muted">Uptime</td><td className="mono">{formatUptime(agent.uptime_seconds)}</td></tr>
              <tr><td className="muted">Error Count</td><td className={agent.error_count > 0 ? 'red' : ''}>{agent.error_count}</td></tr>
              {agent.last_error && <tr><td className="muted">Last Error</td><td className="red">{agent.last_error}</td></tr>}
              {agent.resource_usage && (
                <tr>
                  <td className="muted">Resources</td>
                  <td className="mono dim">
                    CPU: {agent.resource_usage.cpu_percent.toFixed(1)}% |
                    Mem: {agent.resource_usage.memory_mb.toFixed(0)}MB |
                    Disk: {agent.resource_usage.disk_free_mb.toFixed(0)}MB
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      <div className="card">
        <div className="card-title">Capabilities</div>
        <div className="flex gap-8" style={{ flexWrap: 'wrap' }}>
          {(agent.capabilities || []).map(c => (
            <span key={c} className="badge badge-green">{c}</span>
          ))}
          {(!agent.capabilities || agent.capabilities.length === 0) && (
            <span className="dim">None advertised</span>
          )}
        </div>
      </div>

      <div className="tabs">
        {tabs.map(tab => (
          <div
            key={tab}
            className={`tab ${activeTab === tab ? 'active' : ''}`}
            onClick={() => setActiveTab(tab)}
          >
            {tab}
          </div>
        ))}
      </div>

      {activeTab === 'Terminal' && id && <TerminalTab agentId={id} />}
      {activeTab === 'Files' && id && <FilesTab agentId={id} />}
      {activeTab === 'Processes' && id && <ProcessesTab agentId={id} />}
      {activeTab === 'Tunnels' && id && <TunnelsTab agentId={id} />}
      {activeTab === 'MITM' && id && <MITMTab agentId={id} />}
      {activeTab === 'Debug' && id && <DebugTab agentId={id} />}
      {activeTab === 'Screen' && id && <ScreenTab agentId={id} />}
      {activeTab === 'Audit' && (
        <div className="card">
          {audit.length === 0 ? (
            <div className="empty-state">No audit entries for this agent</div>
          ) : (
            <div className="table-container">
              <table>
                <thead>
                  <tr><th>Time</th><th>Action</th><th>Operator</th><th>Result</th></tr>
                </thead>
                <tbody>
                  {audit.map((e, i) => (
                    <tr key={i}>
                      <td className="dim">{new Date(e.timestamp).toLocaleString()}</td>
                      <td className="mono">{e.action}</td>
                      <td className="dim">{e.operator_id || '—'}</td>
                      <td className="dim">{e.result || e.error || '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function formatUptime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
  return `${Math.floor(seconds / 86400)}d ${Math.floor((seconds % 86400) / 3600)}h`
}