import { useState, useEffect, useCallback } from 'react'
import { api } from '../../api/client'
import { Folder, File, ArrowUp, RefreshCw, ChevronLeft, Info } from 'lucide-react'

interface FileEntry { name: string; size: number; is_dir: boolean; mod_time?: string }

export function FilesTab({ agentId }: { agentId: string }) {
  const [path, setPath] = useState('C:\\')
  const [entries, setEntries] = useState<FileEntry[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [selected, setSelected] = useState<FileEntry | null>(null)
  const [fileContent, setFileContent] = useState('')
  const [viewingPath, setViewingPath] = useState('')
  const [showViewer, setShowViewer] = useState(false)

  const listDir = useCallback(async (dir: string) => {
    setLoading(true); setError(''); setSelected(null)
    try {
      const res = await api.fsList(agentId, dir)
      const arr = Array.isArray(res) ? res : (res as { entries?: FileEntry[] })?.entries || []
      setEntries(arr.sort((a, b) => { if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1; return a.name.localeCompare(b.name) }))
      setPath(dir)
    } catch (e) { setError((e as Error).message); setEntries([]) }
    finally { setLoading(false) }
  }, [agentId])

  useEffect(() => { listDir('C:\\') }, [listDir])

  const navigateTo = (entry: FileEntry) => { if (!entry.is_dir) return; const sep = path.endsWith('\\') || path.endsWith('/') ? '' : '\\'; listDir(path + sep + entry.name) }
  const goUp = () => { if (path === 'C:\\' || path === '/' || path === '.') return; const parts = path.split(/[\\/]/); parts.pop(); let p = parts.join('\\'); if (p && !p.includes(':')) p += '\\'; if (!p) p = 'C:\\'; listDir(p) }

  const readFile = async (entry: FileEntry) => {
    if (entry.is_dir) return; setLoading(true); setError('')
    try {
      const sep = path.endsWith('\\') || path.endsWith('/') ? '' : '\\'
      const fp = path + sep + entry.name
      const res = await api.fsRead(agentId, fp)
      const c = typeof res === 'string' ? res : (res as { content?: string })?.content || JSON.stringify(res, null, 2)
      setFileContent(c); setViewingPath(fp); setShowViewer(true)
    } catch (e) { setError((e as Error).message) } finally { setLoading(false) }
  }

  const onFileClick = (entry: FileEntry) => { setSelected(entry); if (!entry.is_dir) readFile(entry) }

  if (showViewer) {
    return (<div><div className="toolbar"><button className="btn btn-sm" onClick={() => setShowViewer(false)}><ChevronLeft size={14} /> Back</button><span className="mono dim">{viewingPath}</span></div><div className="terminal-output" style={{ minHeight: '200px' }}>{fileContent}</div></div>)
  }

  return (
    <div>
      <div className="toolbar">
        <button className="btn btn-sm" onClick={goUp}><ArrowUp size={14} /> Up</button>
        <button className="btn btn-sm" onClick={() => listDir(path)}><RefreshCw size={14} /> Refresh</button>
        <input type="text" value={path} onChange={e => setPath(e.target.value)} onKeyDown={e => e.key === 'Enter' && listDir(path)} className="mono" style={{ flex: 1, padding: '5px 10px', border: '1px solid var(--border-glow)', borderRadius: 'var(--radius)', background: 'var(--bg-input)', color: 'var(--green)', fontFamily: 'var(--font-mono)' }} />
      </div>
      {error && <div className="error-msg">{error}</div>}
      <div className="file-browser">
        <div className="file-pane">
          <div className="file-pane-header"><Folder size={14} /> {path}</div>
          <div className="file-list">
            {loading ? <div className="loading">Loading…</div> :
             entries.length === 0 && !error ? <div className="empty-state">Empty directory</div> :
             <div>
             <div className="file-item file-header" style={{ cursor: 'default', fontWeight: 600, fontSize: 11, color: 'var(--text-muted)', textTransform: 'uppercase', borderBottom: '1px solid var(--border)', marginBottom: 4 }}>
               <span className="file-icon" />
               <span className="file-name">Name</span>
               <span className="file-size">Size</span>
             </div>
             {entries.map((e, i) => (
              <div key={i} className={`file-item ${e.is_dir ? 'dir' : ''} ${selected === e ? 'selected' : ''}`} onClick={() => onFileClick(e)} onDoubleClick={() => navigateTo(e)}>
                <span className="file-icon">{e.is_dir ? <Folder size={14} /> : <File size={14} />}</span>
                <span className="file-name">{e.name}</span>
                <span className="file-size">{e.is_dir ? '—' : formatSize(e.size)}</span>
              </div>
            ))}
            </div>
            }
          </div>
        </div>
        <div className="file-pane">
          <div className="file-pane-header"><Info size={14} /> Details</div>
          <div className="file-list" style={{ padding: '14px' }}>
            {selected ? (
              <div>
                <div style={{ marginBottom: 12 }}>{selected.is_dir ? <Folder size={32} color="var(--green)" /> : <File size={32} />}</div>
                <div className="mono" style={{ fontSize: 14, marginBottom: 10, wordBreak: 'break-all' }}>{selected.name}</div>
                <div style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 4 }}>Type: {selected.is_dir ? 'Directory' : 'File'}</div>
                {!selected.is_dir && <div style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 4 }}>Size: {formatSize(selected.size)}</div>}
                {selected.mod_time && <div style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 4 }}>Modified: {selected.mod_time}</div>}
                <div style={{ marginTop: 14 }}>
                  {!selected.is_dir ? <button className="btn btn-primary btn-sm" onClick={() => readFile(selected)}>View Content</button> : <button className="btn btn-primary btn-sm" onClick={() => navigateTo(selected)}>Open Directory</button>}
                </div>
              </div>
            ) : <div className="empty-state">Select a file to view details</div>}
          </div>
        </div>
      </div>
    </div>
  )
}

function formatSize(b: number): string {
  if (b < 1024) return `${b} B`
  if (b < 1048576) return `${(b / 1024).toFixed(1)} KB`
  if (b < 1073741824) return `${(b / 1048576).toFixed(1)} MB`
  return `${(b / 1073741824).toFixed(1)} GB`
}