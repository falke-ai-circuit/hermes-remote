import { useState, useEffect, useRef } from 'react'
import { api } from '../../api/client'

interface ProcessEntry {
  pid: number
  name: string
  cpu_percent?: number
  memory_mb?: number
  command?: string
}

export function ProcessesTab({ agentId }: { agentId: string }) {
  const [processes, setProcesses] = useState<ProcessEntry[]>([])
  const [error, setError] = useState('')
  const [exe, setExe] = useState('')
  const [args, setArgs] = useState('')
  const [autoRefresh, setAutoRefresh] = useState(false)
  const [loading, setLoading] = useState(false)
  const [filter, setFilter] = useState('')
  const intervalRef = useRef<ReturnType<typeof setInterval>>()

  const listProcs = async () => {
    setLoading(true)
    setError('')
    try {
      const res = await api.procList(agentId)
      const arr = Array.isArray(res) ? res : (res as { processes?: ProcessEntry[] })?.processes || []
      setProcesses(arr.sort((a, b) => a.pid - b.pid))
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  const killProc = async (pid: number) => {
    try {
      await api.procKill(agentId, pid)
      await listProcs()
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const startProc = async () => {
    if (!exe.trim()) return
    try {
      await api.procStart(agentId, exe, args)
      setExe('')
      setArgs('')
      await listProcs()
    } catch (e) {
      setError((e as Error).message)
    }
  }

  useEffect(() => {
    listProcs()
    return () => { if (intervalRef.current) clearInterval(intervalRef.current) }
  }, [])

  useEffect(() => {
    if (autoRefresh) {
      intervalRef.current = setInterval(listProcs, 3000)
    } else if (intervalRef.current) {
      clearInterval(intervalRef.current)
    }
    return () => { if (intervalRef.current) clearInterval(intervalRef.current) }
  }, [autoRefresh])

  const filtered = filter
    ? processes.filter(p =>
        p.name.toLowerCase().includes(filter.toLowerCase()) ||
        String(p.pid).includes(filter))
    : processes

  return (
    <div>
      {error && <div className="error-msg">{error}</div>}

      <div className="toolbar">
        <button className="btn btn-primary btn-sm" onClick={listProcs} disabled={loading}>
          {loading ? 'Loading…' : '⟳ Refresh'}
        </button>
        <button
          className={`btn btn-sm ${autoRefresh ? 'btn-danger' : 'btn-primary'}`}
          onClick={() => setAutoRefresh(!autoRefresh)}
        >
          {autoRefresh ? '⏸ Auto 3s' : '▶ Auto 3s'}
        </button>
        <span className="toolbar-spacer" />
        <input
          type="text"
          value={filter}
          onChange={e => setFilter(e.target.value)}
          placeholder="Filter by name or PID…"
          className="mono"
          style={{ width: 200, padding: '4px 8px', border: '1px solid var(--border)', borderRadius: 'var(--radius)', background: 'var(--bg-input)', color: 'var(--text)' }}
        />
        <span className="dim" style={{ fontSize: 11 }}>{filtered.length} processes</span>
      </div>

      <div className="card" style={{ marginBottom: 14 }}>
        <div className="card-title">Start New Process</div>
        <div className="form-row">
          <div className="form-group">
            <label>Executable</label>
            <input type="text" value={exe} onChange={e => setExe(e.target.value)} placeholder="C:\path\to\exe.exe" />
          </div>
          <div className="form-group">
            <label>Arguments</label>
            <input type="text" value={args} onChange={e => setArgs(e.target.value)} placeholder="-flag value" />
          </div>
          <div className="form-group" style={{ flex: 0 }}>
            <label>&nbsp;</label>
            <button className="btn btn-sm" onClick={startProc}>Start</button>
          </div>
        </div>
      </div>

      {filtered.length > 0 && (
        <div className="table-container">
          <table className="proc-table">
            <thead>
              <tr><th>PID</th><th>Name</th><th>CPU %</th><th>Memory</th><th>Actions</th></tr>
            </thead>
            <tbody>
              {filtered.map(p => (
                <tr key={p.pid} className="proc-row">
                  <td className="mono">{p.pid}</td>
                  <td className="mono">{p.name}</td>
                  <td className="mono dim">{p.cpu_percent?.toFixed(1) || '—'}</td>
                  <td className="mono dim">{p.memory_mb ? `${p.memory_mb.toFixed(0)} MB` : '—'}</td>
                  <td>
                    <button className="btn btn-danger btn-sm" onClick={() => killProc(p.pid)}>Kill</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}