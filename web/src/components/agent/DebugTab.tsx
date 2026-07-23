import { useState } from 'react'
import { api } from '../../api/client'

interface Module {
  name: string
  base: string
  size: number
  path?: string
}

export function DebugTab({ agentId }: { agentId: string }) {
  const [pid, setPid] = useState('')
  const [exePath, setExePath] = useState('')
  const [addr, setAddr] = useState('')
  const [size, setSize] = useState('256')
  const [output, setOutput] = useState('')
  const [hexDump, setHexDump] = useState('')
  const [modules, setModules] = useState<Module[]>([])
  const [error, setError] = useState('')
  const [attached, setAttached] = useState(false)
  const [attachedPid, setAttachedPid] = useState<number | null>(null)

  const attach = async () => {
    setError('')
    setOutput('')
    try {
      await api.debugAttach(agentId, parseInt(pid))
      setAttached(true)
      setAttachedPid(parseInt(pid))
      setOutput(`Attached to PID ${pid}`)
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const attachExe = async () => {
    if (!exePath.trim()) return
    setError('')
    setOutput('')
    try {
      // Start the executable, then attach to it
      const startRes = await api.procStart(agentId, exePath)
      const startedPid = (startRes as { pid?: number })?.pid
      if (startedPid) {
        await api.debugAttach(agentId, startedPid)
        setAttached(true)
        setAttachedPid(startedPid)
        setPid(String(startedPid))
        setOutput(`Started ${exePath} (PID ${startedPid}) and attached`)
      } else {
        setOutput(`Started ${exePath} — could not determine PID for auto-attach`)
      }
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const detach = async () => {
    setError('')
    try {
      await api.debugDetach(agentId)
      setAttached(false)
      setAttachedPid(null)
      setModules([])
      setHexDump('')
      setOutput('Detached')
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const readMem = async () => {
    setError('')
    try {
      const res = await api.debugReadMem(agentId, addr, parseInt(size))
      const data = typeof res === 'string' ? res : JSON.stringify(res, null, 2)
      // Format as hex dump if it's raw bytes
      if (typeof res === 'object' && (res as { data?: string })?.data) {
        setHexDump(formatHexDump((res as { data: string }).data, addr))
        setOutput('')
      } else {
        setOutput(data)
        setHexDump('')
      }
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const getModules = async () => {
    setError('')
    try {
      const res = await api.debugModules(agentId)
      const arr = Array.isArray(res) ? res : (res as { modules?: Module[] })?.modules || []
      setModules(arr)
    } catch (e) {
      setError((e as Error).message)
    }
  }

  return (
    <div>
      {error && <div className="error-msg">{error}</div>}

      {/* Status indicator */}
      <div className="card">
        <div className="card-title">Debug Session Status</div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 14 }}>
          <span className={`status-dot ${attached ? 'active' : 'inactive'}`} style={{ width: 12, height: 12 }} />
          <span style={{ fontSize: 14, fontWeight: 600 }}>
            {attached ? `Attached to PID ${attachedPid}` : 'Not attached'}
          </span>
        </div>

        {/* Load executable */}
        <div className="form-row" style={{ marginBottom: 12 }}>
          <div className="form-group" style={{ flex: 1 }}>
            <label>Load & Attach to Executable</label>
            <input type="text" value={exePath} onChange={e => setExePath(e.target.value)} placeholder="C:\path\to\program.exe" />
          </div>
          <div className="form-group" style={{ flex: 0 }}>
            <label>&nbsp;</label>
            <button className="btn btn-primary btn-sm" onClick={attachExe} disabled={attached || !exePath.trim()}>
              Load & Attach
            </button>
          </div>
        </div>

        {/* Manual PID attach */}
        <div className="form-row">
          <div className="form-group" style={{ flex: 0, width: 120 }}>
            <label>Target PID</label>
            <input type="number" value={pid} onChange={e => setPid(e.target.value)} placeholder="1234" />
          </div>
          <div className="form-group" style={{ flex: 0 }}>
            <label>&nbsp;</label>
            <div className="flex gap-8">
              <button className="btn btn-primary btn-sm" onClick={attach} disabled={attached}>Attach</button>
              <button className="btn btn-danger btn-sm" onClick={detach} disabled={!attached}>Detach</button>
            </div>
          </div>
        </div>
      </div>

      {/* Modules */}
      {attached && (
        <div className="card">
          <div className="card-title">
            Loaded Modules
            <button className="btn btn-sm" style={{ marginLeft: 8 }} onClick={getModules}>Load Modules</button>
          </div>
          {modules.length > 0 ? (
            <div className="table-container">
              <table>
                <thead>
                  <tr><th>Module</th><th>Base Address</th><th>Size</th><th>Path</th></tr>
                </thead>
                <tbody>
                  {modules.map((m, i) => (
                    <tr key={i}>
                      <td className="mono">{m.name}</td>
                      <td className="mono green">{m.base}</td>
                      <td className="mono dim">{formatSize(m.size)}</td>
                      <td className="mono dim">{m.path || '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <div className="empty-state" style={{ padding: 16 }}>Click "Load Modules" to enumerate</div>
          )}
        </div>
      )}

      {/* Memory reader */}
      {attached && (
        <div className="card">
          <div className="card-title">Memory Reader</div>
          <div className="form-row" style={{ marginBottom: 12 }}>
            <div className="form-group" style={{ flex: 1 }}>
              <label>Address (hex)</label>
              <input type="text" value={addr} onChange={e => setAddr(e.target.value)} placeholder="0x400000" className="mono" />
            </div>
            <div className="form-group" style={{ flex: 0, width: 100 }}>
              <label>Size (bytes)</label>
              <input type="number" value={size} onChange={e => setSize(e.target.value)} placeholder="256" />
            </div>
            <div className="form-group" style={{ flex: 0 }}>
              <label>&nbsp;</label>
              <button className="btn btn-sm" onClick={readMem}>Read Memory</button>
            </div>
          </div>
          {hexDump && <div className="hex-dump">{hexDump}</div>}
          {output && <div className="terminal-output">{output}</div>}
        </div>
      )}
    </div>
  )
}

function formatHexDump(data: string, baseAddr: string): string {
  const bytes = data.split('').map(c => c.charCodeAt(0))
  let result = ''
  let addr = parseInt(baseAddr.replace('0x', ''), 16) || 0
  for (let i = 0; i < bytes.length; i += 16) {
    const chunk = bytes.slice(i, i + 16)
    const hex = chunk.map(b => b.toString(16).padStart(2, '0')).join(' ')
    const ascii = chunk.map(b => b >= 32 && b < 127 ? String.fromCharCode(b) : '.').join('')
    result += `0x${(addr + i).toString(16).padStart(8, '0')}  ${hex.padEnd(47)}  ${ascii}\n`
  }
  return result
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1048576).toFixed(1)} MB`
}