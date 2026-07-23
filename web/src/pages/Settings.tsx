import { useState, useEffect } from 'react'
import { api } from '../api/client'
import type { Operator, EnrollmentToken, RevokedAgent, AuditEntry } from '../api/types'

const ROLES = ['admin', 'operator', 'viewer']

export default function Settings() {
  const [operators, setOperators] = useState<Operator[]>([])
  const [tokens, setTokens] = useState<EnrollmentToken[]>([])
  const [revoked, setRevoked] = useState<RevokedAgent[]>([])
  const [audit, setAudit] = useState<AuditEntry[]>([])
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  const [newOpName, setNewOpName] = useState('')
  const [newOpRole, setNewOpRole] = useState('operator')
  const [newOpToken, setNewOpToken] = useState('')

  const [tokenName, setTokenName] = useState('')
  const [tokenTTL, setTokenTTL] = useState('24')

  useEffect(() => { loadAll() }, [])

  const loadAll = async () => {
    try {
      const [ops, toks, rev, aud] = await Promise.all([
        api.listOperators().catch(() => []),
        api.listEnrollmentTokens().catch(() => []),
        api.listRevokedAgents().catch(() => []),
        api.queryAudit({ limit: 50 }).catch(() => []),
      ])
      setOperators(ops || [])
      setTokens(toks || [])
      setRevoked(rev || [])
      setAudit(aud || [])
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const createOperator = async () => {
    setError('')
    setSuccess('')
    try {
      await api.createOperator(newOpName, newOpRole, newOpToken || undefined as unknown as string)
      setNewOpName('')
      setNewOpToken('')
      setSuccess('Operator created')
      loadAll()
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const deleteOperator = async (id: string) => {
    try {
      await api.deleteOperator(id)
      loadAll()
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const createToken = async () => {
    setError('')
    setSuccess('')
    try {
      await api.createEnrollmentToken(tokenName, parseInt(tokenTTL))
      setTokenName('')
      setSuccess('Enrollment token created')
      loadAll()
    } catch (e) {
      setError((e as Error).message)
    }
  }

  return (
    <div>
      <div className="page-header">
        <h1>Settings</h1>
        <p>Server configuration, operators, enrollment tokens, and audit log</p>
      </div>

      {error && <div className="error-msg">{error}</div>}
      {success && <div className="success-msg">{success}</div>}

      {/* Operators */}
      <div className="card">
        <div className="card-title">Operators</div>
        <div className="form-row mb-16">
          <div className="form-group">
            <label>Name</label>
            <input type="text" value={newOpName} onChange={e => setNewOpName(e.target.value)} placeholder="operator-name" />
          </div>
          <div className="form-group">
            <label>Role</label>
            <select value={newOpRole} onChange={e => setNewOpRole(e.target.value)}>
              {ROLES.map(r => <option key={r} value={r}>{r}</option>)}
            </select>
          </div>
          <div className="form-group">
            <label>Token (optional)</label>
            <input type="text" value={newOpToken} onChange={e => setNewOpToken(e.target.value)} placeholder="auto-generated" />
          </div>
          <div className="form-group">
            <label>&nbsp;</label>
            <button className="btn btn-primary btn-sm" onClick={createOperator}>Add</button>
          </div>
        </div>
        {operators.length === 0 ? (
          <div className="empty-state">No operators configured — legacy token auth active</div>
        ) : (
          <div className="table-container">
            <table>
              <thead><tr><th>ID</th><th>Name</th><th>Role</th><th>Created</th><th>Last Seen</th><th>Actions</th></tr></thead>
              <tbody>
                {operators.map(o => (
                  <tr key={o.id}>
                    <td className="mono dim">{o.id.slice(0, 12)}…</td>
                    <td>{o.name}</td>
                    <td><span className={`badge ${o.role === 'admin' ? 'badge-green' : o.role === 'operator' ? 'badge-blue' : 'badge-gray'}`}>{o.role}</span></td>
                    <td className="dim">{o.created_at ? new Date(o.created_at).toLocaleString() : '—'}</td>
                    <td className="dim">{o.last_seen ? new Date(o.last_seen).toLocaleString() : '—'}</td>
                    <td>
                      <button className="btn btn-danger btn-sm" onClick={() => deleteOperator(o.id)}>Delete</button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Enrollment Tokens */}
      <div className="card">
        <div className="card-title">Enrollment Tokens</div>
        <div className="form-row mb-16">
          <div className="form-group">
            <label>Agent Name</label>
            <input type="text" value={tokenName} onChange={e => setTokenName(e.target.value)} placeholder="new-agent" />
          </div>
          <div className="form-group">
            <label>TTL (hours)</label>
            <input type="number" value={tokenTTL} onChange={e => setTokenTTL(e.target.value)} />
          </div>
          <div className="form-group">
            <label>&nbsp;</label>
            <button className="btn btn-primary btn-sm" onClick={createToken}>Generate</button>
          </div>
        </div>
        {tokens.length === 0 ? (
          <div className="empty-state">No enrollment tokens</div>
        ) : (
          <div className="table-container">
            <table>
              <thead><tr><th>Token</th><th>Agent Name</th><th>Created</th><th>Expires</th><th>Used</th></tr></thead>
              <tbody>
                {tokens.map(t => (
                  <tr key={t.token}>
                    <td className="mono dim">{t.token.slice(0, 20)}…</td>
                    <td>{t.agent_name}</td>
                    <td className="dim">{t.created_at ? new Date(t.created_at).toLocaleString() : '—'}</td>
                    <td className="dim">{t.expires_at ? new Date(t.expires_at).toLocaleString() : '—'}</td>
                    <td>{t.used ? <span className="badge badge-gray">used</span> : <span className="badge badge-green">available</span>}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Revoked Agents */}
      {revoked.length > 0 && (
        <div className="card">
          <div className="card-title">Revoked Agents</div>
          <div className="table-container">
            <table>
              <thead><tr><th>Agent ID</th><th>Revoked At</th><th>Reason</th></tr></thead>
              <tbody>
                {revoked.map(r => (
                  <tr key={r.agent_id}>
                    <td className="mono dim">{r.agent_id}</td>
                    <td className="dim">{r.revoked_at ? new Date(r.revoked_at).toLocaleString() : '—'}</td>
                    <td className="red">{r.reason}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Audit Log */}
      <div className="card">
        <div className="card-title">Recent Audit Log (last 50)</div>
        {audit.length === 0 ? (
          <div className="empty-state">No audit entries</div>
        ) : (
          <div className="table-container">
            <table>
              <thead><tr><th>Time</th><th>Agent</th><th>Action</th><th>Operator</th></tr></thead>
              <tbody>
                {audit.map((e, i) => (
                  <tr key={i}>
                    <td className="dim">{e.timestamp ? new Date(e.timestamp).toLocaleString() : '—'}</td>
                    <td className="mono dim">{e.agent_id?.slice(0, 12) || '—'}…</td>
                    <td className="mono">{e.action}</td>
                    <td className="dim">{e.operator_id || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}