import { useState, useEffect } from 'react'
import { api } from '../api/client'
import type { Task } from '../api/types'
import { StatusBadge } from '../components/StatusBadge'

const CMD_TYPES = ['exec', 'fs-list', 'fs-read', 'fs-write', 'fs-stat', 'proc-list', 'proc-kill', 'proc-start', 'capture']
const SCHEDULE_TYPES = [
  { value: 'once', label: 'Once (immediate)' },
  { value: 'delayed', label: 'Delayed (after N seconds)' },
  { value: 'recurring', label: 'Recurring (every N seconds)' },
]

export default function Tasks() {
  const [tasks, setTasks] = useState<Task[]>([])
  const [error, setError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({
    agent_id: '',
    command_type: 'exec',
    params: '',
    schedule_type: 'once',
    delay_seconds: 10,
    interval_seconds: 60,
  })

  const load = async () => {
    try {
      const t = await api.listTasks()
      setTasks(t || [])
    } catch (e) {
      setError((e as Error).message)
    }
  }

  useEffect(() => { load() }, [])

  const createTask = async () => {
    setError('')
    try {
      let params: unknown = {}
      if (form.params.trim()) {
        try { params = JSON.parse(form.params) } catch { params = { command: form.params } }
      }
      const schedule: { type: string; delay_seconds?: number; interval_seconds?: number } = { type: form.schedule_type }
      if (form.schedule_type === 'delayed') schedule.delay_seconds = form.delay_seconds
      if (form.schedule_type === 'recurring') schedule.interval_seconds = form.interval_seconds

      await api.createTask({
        agent_id: form.agent_id,
        command_type: form.command_type,
        params,
        schedule,
      })
      setShowForm(false)
      load()
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const cancelTask = async (id: string) => {
    try {
      await api.cancelTask(id)
      load()
    } catch (e) {
      setError((e as Error).message)
    }
  }

  return (
    <div>
      <div className="page-header">
        <h1>Tasks</h1>
        <p>Scheduled, delayed, and recurring tasks for agents</p>
      </div>

      {error && <div className="error-msg">{error}</div>}

      <div className="flex gap-8 mb-16">
        <button className="btn btn-primary btn-sm" onClick={() => setShowForm(!showForm)}>
          {showForm ? 'Cancel' : '+ New Task'}
        </button>
        <button className="btn btn-sm" onClick={load}>Refresh</button>
      </div>

      {showForm && (
        <div className="card">
          <div className="card-title">Create Task</div>
          <div className="form-group">
            <label>Agent ID</label>
            <input type="text" value={form.agent_id} onChange={e => setForm({ ...form, agent_id: e.target.value })} placeholder="agent-uuid" />
          </div>
          <div className="form-row">
            <div className="form-group">
              <label>Command Type</label>
              <select value={form.command_type} onChange={e => setForm({ ...form, command_type: e.target.value })}>
                {CMD_TYPES.map(c => <option key={c} value={c}>{c}</option>)}
              </select>
            </div>
            <div className="form-group">
              <label>Schedule</label>
              <select value={form.schedule_type} onChange={e => setForm({ ...form, schedule_type: e.target.value })}>
                {SCHEDULE_TYPES.map(s => <option key={s.value} value={s.value}>{s.label}</option>)}
              </select>
            </div>
          </div>
          {form.schedule_type === 'delayed' && (
            <div className="form-group">
              <label>Delay (seconds)</label>
              <input type="number" value={form.delay_seconds} onChange={e => setForm({ ...form, delay_seconds: parseInt(e.target.value) || 0 })} />
            </div>
          )}
          {form.schedule_type === 'recurring' && (
            <div className="form-group">
              <label>Interval (seconds)</label>
              <input type="number" value={form.interval_seconds} onChange={e => setForm({ ...form, interval_seconds: parseInt(e.target.value) || 0 })} />
            </div>
          )}
          <div className="form-group">
            <label>Parameters (JSON or command string)</label>
            <textarea value={form.params} onChange={e => setForm({ ...form, params: e.target.value })} placeholder='{"command": "whoami"}' />
          </div>
          <button className="btn btn-primary" onClick={createTask}>Create Task</button>
        </div>
      )}

      <div className="card">
        {tasks.length === 0 ? (
          <div className="empty-state">
            <p>No tasks</p>
            <button className="btn btn-primary btn-sm" style={{ marginTop: 12 }} onClick={() => setShowForm(true)}>+ New Task</button>
          </div>
        ) : (
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>Task ID</th><th>Agent</th><th>Type</th><th>Schedule</th>
                  <th>Status</th><th>Created</th><th>Completed</th><th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {tasks.map(t => (
                  <tr key={t.id}>
                    <td className="mono dim">{t.id.slice(0, 8)}…</td>
                    <td className="mono dim">{t.agent_id.slice(0, 12)}…</td>
                    <td>{t.command_type}</td>
                    <td>{t.schedule.type}</td>
                    <td><StatusBadge status={t.status} /></td>
                    <td className="dim">{t.created_at ? new Date(t.created_at).toLocaleString() : '—'}</td>
                    <td className="dim">{t.completed_at ? new Date(t.completed_at).toLocaleString() : '—'}</td>
                    <td>
                      {(t.status === 'pending' || t.status === 'queued' || t.status === 'running') && (
                        <button className="btn btn-danger btn-sm" onClick={() => cancelTask(t.id)}>Cancel</button>
                      )}
                    </td>
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