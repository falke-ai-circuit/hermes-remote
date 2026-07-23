import { useState } from 'react'
import { api } from '../../api/client'

interface ProcessEntry {
  pid: number
  name: string
  cpu_percent?: number
  memory_mb?: number
}

export function ProcessesTab({ agentId }: { agentId: string }) {
  const [processes, setProcesses] = useState<ProcessEntry[]>([])
  const [error, setError] = useState('')
  const [exe, setExe] = useState('')
  const [args, setArgs] = useState('')

  const listProcs = async () => {
    setError('')
    try {
      const res = await api.procList(agentId)
      const arr = Array.isArray(res) ? res : (res as { processes?: ProcessEntry[] })?.processes || []
      setProcesses(arr)
    } catch (e) {
      setError((e as Error).message)
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

  return (
    <div className="card">
      {error && <div className="error-msg">{error}</div>}

      <div className="flex gap-8 mb-16">
        <button className="btn btn-primary btn-sm" onClick={listProcs}>List Processes</button>
      </div>

      <div className="card-title">Start New Process</div>
      <div className="form-row mb-16">
        <div className="form-group">
          <label>Executable</label>
          <input type="text" value={exe} onChange={e => setExe(e.target.value)} placeholder="C:\\path\\to\\exe.exe" />
        </div>
        <div className="form-group">
          <label>Arguments</label>
          <input type="text" value={args} onChange={e => setArgs(e.target.value)} placeholder="-flag value" />
        </div>
      </div>
      <button className="btn btn-sm" onClick={startProc}>Start Process</button>

      {processes.length > 0 && (
        <div className="table-container mt-16">
          <table>
            <thead>
              <tr><th>PID</th><th>Name</th><th>CPU %</th><th>Memory (MB)</th><th>Actions</th></tr>
            </thead>
            <tbody>
              {processes.map(p => (
                <tr key={p.pid}>
                  <td className="mono">{p.pid}</td>
                  <td className="mono">{p.name}</td>
                  <td className="mono dim">{p.cpu_percent?.toFixed(1) || '—'}</td>
                  <td className="mono dim">{p.memory_mb?.toFixed(0) || '—'}</td>
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