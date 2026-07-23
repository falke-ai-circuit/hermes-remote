import { useState, useEffect, useCallback } from 'react'
import { api } from '../../api/client'

interface FileEntry {
  name: string
  size: number
  is_dir: boolean
  mod_time?: string
}

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
    setLoading(true)
    setError('')
    setSelected(null)
    try {
      const res = await api.fsList(agentId, dir)
      const arr = Array.isArray(res) ? res : (res as { entries?: FileEntry[] })?.entries || []
      setEntries(arr.sort((a, b) => {
        if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1
        return a.name.localeCompare(b.name)
      }))
      setPath(dir)
    } catch (e) {
      setError((e as Error).message)
      setEntries([])
    } finally {
      setLoading(false)
    }
  }, [agentId])

  useEffect(() => { listDir('C:\\') }, [listDir])

  const navigateTo = (entry: FileEntry) => {
    if (!entry.is_dir) return
    const sep = path.endsWith('\\') || path.endsWith('/') ? '' : '\\'
    const newPath = path + sep + entry.name
    listDir(newPath)
  }

  const goUp = () => {
    if (path === 'C:\\' || path === '/' || path === '.') return
    const parts = path.split(/[\\/]/)
    parts.pop()
    let parent = parts.join('\\')
    if (parent && !parent.includes(':')) parent = parent + '\\'
    if (!parent) parent = 'C:\\'
    listDir(parent)
  }

  const readFile = async (entry: FileEntry) => {
    if (entry.is_dir) return
    setLoading(true)
    setError('')
    try {
      const sep = path.endsWith('\\') || path.endsWith('/') ? '' : '\\'
      const filePath = path + sep + entry.name
      const res = await api.fsRead(agentId, filePath)
      const content = typeof res === 'string' ? res
        : (res as { content?: string })?.content
        || JSON.stringify(res, null, 2)
      setFileContent(content)
      setViewingPath(filePath)
      setShowViewer(true)
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  const onFileClick = (entry: FileEntry) => {
    setSelected(entry)
    if (!entry.is_dir) readFile(entry)
  }

  if (showViewer) {
    return (
      <div>
        <div className="toolbar">
          <button className="btn btn-sm" onClick={() => setShowViewer(false)}>← Back</button>
          <span className="mono dim">{viewingPath}</span>
        </div>
        <div className="terminal-output" style={{ minHeight: '200px' }}>{fileContent}</div>
      </div>
    )
  }

  return (
    <div>
      <div className="toolbar">
        <button className="btn btn-sm" onClick={goUp}>↑ Up</button>
        <button className="btn btn-sm" onClick={() => listDir(path)}>⟳ Refresh</button>
        <input
          type="text"
          value={path}
          onChange={e => setPath(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && listDir(path)}
          className="mono"
          style={{ flex: 1, padding: '4px 8px', border: '1px solid var(--border-glow)', borderRadius: 'var(--radius)', background: 'var(--bg-input)', color: 'var(--probe-green)' }}
        />
      </div>

      {error && <div className="error-msg">{error}</div>}

      <div className="file-browser">
        {/* Left pane - directory listing */}
        <div className="file-pane">
          <div className="file-pane-header">
            <span>📁 {path}</span>
          </div>
          <div className="file-list">
            {loading ? (
              <div className="loading">Loading…</div>
            ) : entries.length === 0 && !error ? (
              <div className="empty-state">Empty directory</div>
            ) : (
              entries.map((e, i) => (
                <div
                  key={i}
                  className={`file-item ${e.is_dir ? 'dir' : ''} ${selected === e ? 'selected' : ''}`}
                  onClick={() => onFileClick(e)}
                  onDoubleClick={() => navigateTo(e)}
                >
                  <span className="file-icon">{e.is_dir ? '📁' : '📄'}</span>
                  <span className="file-name">{e.name}</span>
                  <span className="file-size">{e.is_dir ? '—' : formatSize(e.size)}</span>
                </div>
              ))
            )}
          </div>
        </div>

        {/* Right pane - file info / preview */}
        <div className="file-pane">
          <div className="file-pane-header">
            <span>ℹ Details</span>
          </div>
          <div className="file-list" style={{ padding: '12px' }}>
            {selected ? (
              <div>
                <div style={{ marginBottom: 12 }}>
                  <span style={{ fontSize: 28 }}>{selected.is_dir ? '📁' : '📄'}</span>
                </div>
                <div className="mono" style={{ fontSize: 14, marginBottom: 8, wordBreak: 'break-all' }}>{selected.name}</div>
                <div style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 4 }}>
                  Type: {selected.is_dir ? 'Directory' : 'File'}
                </div>
                {!selected.is_dir && (
                  <div style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 4 }}>
                    Size: {formatSize(selected.size)}
                  </div>
                )}
                {selected.mod_time && (
                  <div style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 4 }}>
                    Modified: {selected.mod_time}
                  </div>
                )}
                <div style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 12 }}>
                  Path: {path}{selected.name}
                </div>
                {!selected.is_dir && (
                  <button className="btn btn-primary btn-sm" onClick={() => readFile(selected)}>
                    View Content
                  </button>
                )}
                {selected.is_dir && (
                  <button className="btn btn-primary btn-sm" onClick={() => navigateTo(selected)}>
                    Open Directory
                  </button>
                )}
              </div>
            ) : (
              <div className="empty-state">Select a file to view details</div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1073741824) return `${(bytes / 1048576).toFixed(1)} MB`
  return `${(bytes / 1073741824).toFixed(1)} GB`
}