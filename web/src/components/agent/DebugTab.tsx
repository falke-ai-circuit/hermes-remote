import { useState } from 'react'
import { api } from '../../api/client'
import { Bug, Link2, Unlink, Cpu, MemoryStick, FileSearch } from 'lucide-react'

interface Module { name: string; base: string; size: number; path?: string }

export function DebugTab({ agentId }: { agentId: string }) {
  const [pid, setPid] = useState(''); const [exePath, setExePath] = useState('')
  const [addr, setAddr] = useState(''); const [size, setSize] = useState('256')
  const [output, setOutput] = useState(''); const [hexDump, setHexDump] = useState('')
  const [modules, setModules] = useState<Module[]>([])
  const [error, setError] = useState(''); const [attached, setAttached] = useState(false)
  const [attachedPid, setAttachedPid] = useState<number | null>(null)
  const [debugId, setDebugId] = useState(''); const [baseAddr, setBaseAddr] = useState(0)

  const attach = async () => { setError(''); setOutput(''); try { const res = await api.debugAttach(agentId, parseInt(pid)); const d = (typeof res === 'object' ? res : {}) as { debug_id?: string; base_addr?: number }; setAttached(true); setAttachedPid(parseInt(pid)); setDebugId(d.debug_id || ''); setBaseAddr(d.base_addr || 0); setOutput(`Attached to PID ${pid}`) } catch (e) { setError((e as Error).message) } }
  const attachExe = async () => { if (!exePath.trim()) return; setError(''); setOutput(''); try { const sr = await api.procStart(agentId, exePath); const sp = (typeof sr === 'object' ? sr : {}) as { pid?: number }; if (sp.pid) { await api.debugAttach(agentId, sp.pid); setAttached(true); setAttachedPid(sp.pid); setPid(String(sp.pid)); setOutput(`Started ${exePath} (PID ${sp.pid}) and attached`) } else { setOutput(`Started ${exePath} — could not determine PID`) } } catch (e) { setError((e as Error).message) } }
  const detach = async () => { setError(''); try { await api.debugDetach(agentId); setAttached(false); setAttachedPid(null); setModules([]); setHexDump(''); setOutput('Detached') } catch (e) { setError((e as Error).message) } }
  const readMem = async () => { setError(''); try { const res = await api.debugReadMem(agentId, String(baseAddr), parseInt(size)); const d = (typeof res === 'object' ? res : {}) as { data?: string; hex_data?: string }; if (d.hex_data) { setHexDump(d.hex_data); setOutput('') } else { setOutput(JSON.stringify(res, null, 2)); setHexDump('') } } catch (e) { setError((e as Error).message) } }
  const getModules = async () => { setError(''); try { const res = await api.debugModules(agentId); const arr = Array.isArray(res) ? res : (res as { modules?: Module[] })?.modules || []; setModules(arr) } catch (e) { setError((e as Error).message) } }

  return (
    <div>
      {error && <div className="error-msg">{error}</div>}
      <div className="card">
        <div className="card-title"><Bug size={12} style={{ display: 'inline' }} /> Debug Session Status</div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 14 }}>
          <span className={`status-dot ${attached ? 'active' : 'inactive'}`} style={{ width: 12, height: 12 }} />
          <span style={{ fontSize: 14, fontWeight: 600 }}>{attached ? `Attached to PID ${attachedPid}` : 'Not attached'}</span>
        </div>
        <div className="form-row" style={{ marginBottom: 12 }}>
          <div className="form-group" style={{ flex: 1 }}><label>Load & Attach to Executable</label><input type="text" value={exePath} onChange={e => setExePath(e.target.value)} placeholder="C:\path\to\program.exe" /></div>
          <div className="form-group" style={{ flex: 0 }}><label>&nbsp;</label><button className="btn btn-primary btn-sm" onClick={attachExe} disabled={attached || !exePath.trim()}><FileSearch size={14} /> Load & Attach</button></div>
        </div>
        <div className="form-row">
          <div className="form-group" style={{ flex: 0, width: 120 }}><label>Target PID</label><input type="number" value={pid} onChange={e => setPid(e.target.value)} placeholder="1234" /></div>
          <div className="form-group" style={{ flex: 0 }}><label>&nbsp;</label><div className="flex gap-8"><button className="btn btn-primary btn-sm" onClick={attach} disabled={attached}><Link2 size={14} /> Attach</button><button className="btn btn-danger btn-sm" onClick={detach} disabled={!attached}><Unlink size={14} /> Detach</button></div></div>
        </div>
      </div>
      {attached && (
        <>
          <div className="card">
            <div className="card-title"><Cpu size={12} style={{ display: 'inline' }} /> Loaded Modules <button className="btn btn-sm" style={{ marginLeft: 8 }} onClick={getModules}><FileSearch size={14} /> Load</button></div>
            {modules.length > 0 ? (
              <div className="table-container"><table>
                <thead><tr><th>Module</th><th>Base Address</th><th>Size</th><th>Path</th></tr></thead>
                <tbody>{modules.map((m, i) => (<tr key={i}><td className="mono">{m.name}</td><td className="mono green">{m.base}</td><td className="mono dim">{formatSize(m.size)}</td><td className="mono dim">{m.path || '—'}</td></tr>))}</tbody>
              </table></div>
            ) : <div className="empty-state" style={{ padding: 16 }}>Click "Load" to enumerate</div>}
          </div>
          <div className="card">
            <div className="card-title"><MemoryStick size={12} style={{ display: 'inline' }} /> Memory Reader</div>
            <div className="form-row" style={{ marginBottom: 12 }}>
              <div className="form-group" style={{ flex: 1 }}><label>Address (uint64 from attach)</label><input type="text" value={baseAddr || addr} onChange={e => setAddr(e.target.value)} placeholder="0" className="mono" /></div>
              <div className="form-group" style={{ flex: 0, width: 100 }}><label>Size (bytes)</label><input type="number" value={size} onChange={e => setSize(e.target.value)} /></div>
              <div className="form-group" style={{ flex: 0 }}><label>&nbsp;</label><button className="btn btn-sm" onClick={readMem}><MemoryStick size={14} /> Read</button></div>
            </div>
            {hexDump && <div className="hex-dump">{hexDump}</div>}
            {output && <div className="terminal-output">{output}</div>}
          </div>
        </>
      )}
    </div>
  )
}

function formatSize(b: number): string {
  if (b < 1024) return `${b} B`
  if (b < 1048576) return `${(b / 1024).toFixed(1)} KB`
  return `${(b / 1048576).toFixed(1)} MB`
}