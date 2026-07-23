import { useState } from 'react'
import { api } from '../../api/client'

interface FileEntry {
  name: string
  size: number
  is_dir: boolean
  mod_time?: string
}

export function FilesTab({ agentId }: { agentId: string }) {
  const [path, setPath] = useState('.')
  const [entries, setEntries] = useState<FileEntry[]>([])
  const [error, setError] = useState('')
  const [fileContent, setFileContent] = useState('')
  const [viewingPath, setViewingPath] = useState('')
  const [loading, setLoading] = useState(false)

  const listDir = async (dir: string) => {
    setLoading(true)
    setError('')
    setFileContent('')
    try {
      const res = await api.fsList(agentId, dir)
      const arr = Array.isArray(res) ? res : (res as { entries?: FileEntry[] })?.entries || []
      setEntries(arr)
      setPath(dir)
    } catch (e) {
      setError((e as Error).message)
      setEntries([])
    } finally {
      setLoading(false)
    }
  }

  const readFile = async (filePath: string) => {
    setLoading(true)
    setError('')
    try {
      const res = await api.fsRead(agentId, filePath)
      const content = typeof res === 'string' ? res : (res as { content?: string })?.content || JSON.stringify(res, null, 2)
      setFileContent(content)
      setViewingPath(filePath)
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  const navigateTo = (dir: string) => {
    const newPath = path === '.' ? dir : `${path}/${dir}`
    listDir(newPath)
  }

  const goUp = () => {
    if (path === '.' || path === '/' || path === '') return
    const parts = path.split('/')
    parts.pop()
    listDir(parts.join('/') || '.')
  }

  return (
    <div className="card">
      <div className="flex gap-8 mb-16" style={{ alignItems: 'center' }}>
        <button className="btn btn-sm" onClick={goUp}>↑ Up</button>
        <button className="btn btn-sm" onClick={() => listDir(path)}>⟳ Refresh</button>
        <input
          type="text"
          value={path}
          onChange={e => setPath(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && listDir(path)}
          className="mono"
          style={{ flex: 1, padding: '4px 8px', border: '1px solid var(--border)', borderRadius: 'var(--radius)', background: 'var(--bg)', color: 'var(--text)' }}
        />
      </div>

      {error && <div className="error-msg">{error}</div>}

      {fileContent ? (
        <div>
          <div className="flex gap-8 mb-16" style={{ alignItems: 'center' }}>
            <button className="btn btn-sm" onClick={() => { setFileContent(''); setViewingPath('') }}>← Back</button>
            <span className="mono dim">{viewingPath}</span>
          </div>
          <div className="terminal-output" style={{ minHeight: '200px' }}>{fileContent}</div>
        </div>
      ) : (
        <div className="table-container">
          {loading ? (
            <div className="loading">Loading…</div>
          ) : entries.length === 0 && !error ? (
            <div className="empty-state">Click Refresh to list files</div>
          ) : (
            <table>
              <thead>
                <tr><th>Name</th><th>Size</th><th>Type</th><th>Modified</th><th>Actions</th></tr>
              </thead>
              <tbody>
                {entries.map((e, i) => (
                  <tr key={i} className={e.is_dir ? 'clickable' : ''} onClick={e.is_dir ? () => navigateTo(e.name) : undefined}>
                    <td className="mono">{e.is_dir ? '📁 ' : '📄 '}{e.name}</td>
                    <td className="mono dim">{e.is_dir ? '—' : formatSize(e.size)}</td>
                    <td>{e.is_dir ? 'dir' : 'file'}</td>
                    <td className="dim">{e.mod_time || '—'}</td>
                    <td>
                      {!e.is_dir && (
                        <button className="btn btn-sm" onClick={(ev) => { ev.stopPropagation(); readFile(`${path === '.' ? '' : path + '/'}${e.name}`) }}>
                          Read
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </div>
  )
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1048576).toFixed(1)} MB`
}