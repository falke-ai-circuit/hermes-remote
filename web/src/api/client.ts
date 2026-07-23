import type {
  APIResponse, AgentRecord, HealthInfo, BuildConfig, Profile, Task,
  Operator, EnrollmentToken, AuditEntry, RevokedAgent,
} from './types'

const BASE = '/api/v1'

async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  })
  const body: APIResponse<T> = await res.json()
  if (!body.ok) {
    throw new Error(body.error?.message || `HTTP ${res.status}`)
  }
  return body.data as T
}

export const api = {
  // Health
  getHealth: () => apiFetch<HealthInfo>('/health'),

  // Agents
  listAgents: () => apiFetch<AgentRecord[]>('/agents'),
  getAgent: (id: string) => apiFetch<AgentRecord>(`/agents/${id}`),
  deleteAgent: (id: string) =>
    apiFetch<{ removed: string }>(`/agents/${id}`, { method: 'DELETE' }),
  getAgentHealth: (id: string) => apiFetch<AgentRecord>(`/agents/${id}/health`),
  getAgentAudit: (id: string) => apiFetch<AuditEntry[]>(`/agents/${id}/audit`),

  // Agent commands (all POST)
  execCmd: (id: string, cmd: string) =>
    apiFetch<unknown>(`/agents/${id}/exec`, {
      method: 'POST',
      body: JSON.stringify({ command: cmd }),
    }),
  fsList: (id: string, dir: string) =>
    apiFetch<unknown>(`/agents/${id}/fs-list`, {
      method: 'POST',
      body: JSON.stringify({ path: dir }),
    }),
  fsRead: (id: string, path: string) =>
    apiFetch<unknown>(`/agents/${id}/fs-read`, {
      method: 'POST',
      body: JSON.stringify({ path }),
    }),
  fsWrite: (id: string, path: string, content: string) =>
    apiFetch<unknown>(`/agents/${id}/fs-write`, {
      method: 'POST',
      body: JSON.stringify({ path, content, encoding: 'base64', data: btoa(content) }),
    }),
  fsStat: (id: string, path: string) =>
    apiFetch<unknown>(`/agents/${id}/fs-stat`, {
      method: 'POST',
      body: JSON.stringify({ path }),
    }),
  fsHash: (id: string, path: string) =>
    apiFetch<unknown>(`/agents/${id}/fs-hash`, {
      method: 'POST',
      body: JSON.stringify({ path }),
    }),
  procList: (id: string) =>
    apiFetch<unknown>(`/agents/${id}/proc-list`, { method: 'POST' }),
  procKill: (id: string, pid: number) =>
    apiFetch<unknown>(`/agents/${id}/proc-kill`, {
      method: 'POST',
      body: JSON.stringify({ pid }),
    }),
  procStart: (id: string, exe: string, args?: string) =>
    apiFetch<unknown>(`/agents/${id}/proc-start`, {
      method: 'POST',
      body: JSON.stringify({ executable: exe, args: args || '' }),
    }),
  capture: (id: string) =>
    apiFetch<unknown>(`/agents/${id}/capture`, { method: 'POST' }),
  tunnelOpen: (id: string, port: number, target: string) =>
    apiFetch<unknown>(`/agents/${id}/tunnel`, {
      method: 'POST',
      body: JSON.stringify({ local_port: port, target_address: target }),
    }),
  tunnelClose: (id: string, tunnelId: string) =>
    apiFetch<unknown>(`/agents/${id}/tunnel-close`, {
      method: 'POST',
      body: JSON.stringify({ tunnel_id: tunnelId }),
    }),
  mitmStart: (id: string, target: string, port: number) =>
    apiFetch<unknown>(`/agents/${id}/mitm-start`, {
      method: 'POST',
      body: JSON.stringify({ target_address: target, target_port: port }),
    }),
  mitmStop: (id: string) =>
    apiFetch<unknown>(`/agents/${id}/mitm-stop`, { method: 'POST' }),
  mitmTraffic: (id: string) =>
    apiFetch<unknown>(`/agents/${id}/mitm-traffic`, { method: 'POST' }),
  debugAttach: (id: string, pid: number) =>
    apiFetch<unknown>(`/agents/${id}/debug-attach`, {
      method: 'POST',
      body: JSON.stringify({ pid }),
    }),
  debugDetach: (id: string) =>
    apiFetch<unknown>(`/agents/${id}/debug-detach`, { method: 'POST' }),
  debugReadMem: (id: string, addr: string, size: number) =>
    apiFetch<unknown>(`/agents/${id}/debug-read-mem`, {
      method: 'POST',
      body: JSON.stringify({ address: addr, size }),
    }),
  debugModules: (id: string) =>
    apiFetch<unknown>(`/agents/${id}/debug-modules`, { method: 'POST' }),

  // Builds
  listBuilds: () => apiFetch<BuildConfig[]>('/builds'),
  createBuild: (cfg: BuildConfig) =>
    apiFetch<BuildConfig>('/builds', {
      method: 'POST',
      body: JSON.stringify(cfg),
    }),
  getBuild: (id: string) => apiFetch<BuildConfig>(`/builds/${id}`),
  downloadBuildUrl: (id: string) => `${BASE}/builds/${id}/download`,
  deleteBuild: (id: string) =>
    apiFetch<{ deleted: string }>(`/builds/${id}`, { method: 'DELETE' }),

  // Profiles
  listProfiles: () => apiFetch<Profile[]>('/profiles'),
  createProfile: (p: Profile) =>
    apiFetch<Profile>('/profiles', {
      method: 'POST',
      body: JSON.stringify(p),
    }),
  getProfile: (id: string) => apiFetch<Profile>(`/profiles/${id}`),
  deleteProfile: (id: string) =>
    apiFetch<{ deleted: string }>(`/profiles/${id}`, { method: 'DELETE' }),

  // Tasks
  listTasks: (agentId?: string, status?: string) => {
    const params = new URLSearchParams()
    if (agentId) params.set('agent_id', agentId)
    if (status) params.set('status', status)
    const q = params.toString()
    return apiFetch<Task[]>(`/tasks${q ? '?' + q : ''}`)
  },
  createTask: (task: {
    agent_id: string
    command_type: string
    params: unknown
    schedule: { type: string; delay_seconds?: number; interval_seconds?: number }
  }) =>
    apiFetch<Task>('/tasks', {
      method: 'POST',
      body: JSON.stringify(task),
    }),
  getTask: (id: string) => apiFetch<Task>(`/tasks/${id}`),
  cancelTask: (id: string) =>
    apiFetch<{ cancelled: string }>(`/tasks/${id}`, { method: 'DELETE' }),

  // Operators
  listOperators: () => apiFetch<Operator[]>('/operators'),
  createOperator: (name: string, role: string, token: string) =>
    apiFetch<Operator>('/operators', {
      method: 'POST',
      body: JSON.stringify({ name, role, token }),
    }),
  deleteOperator: (id: string) =>
    apiFetch<{ deleted: string }>(`/operators/${id}`, { method: 'DELETE' }),

  // Enrollment tokens
  listEnrollmentTokens: () => apiFetch<EnrollmentToken[]>('/enrollment-tokens'),
  createEnrollmentToken: (agentName: string, ttlHours: number) =>
    apiFetch<EnrollmentToken>('/enrollment-tokens', {
      method: 'POST',
      body: JSON.stringify({ agent_name: agentName, ttl_hours: ttlHours }),
    }),

  // Revoked agents
  listRevokedAgents: () => apiFetch<RevokedAgent[]>('/agents/revoked'),

  // Audit
  queryAudit: (filter: { agent_id?: string; operator_id?: string; action?: string; limit?: number }) => {
    const params = new URLSearchParams()
    if (filter.agent_id) params.set('agent_id', filter.agent_id)
    if (filter.operator_id) params.set('operator_id', filter.operator_id)
    if (filter.action) params.set('action', filter.action)
    if (filter.limit) params.set('limit', String(filter.limit))
    const q = params.toString()
    return apiFetch<AuditEntry[]>(`/audit${q ? '?' + q : ''}`)
  },
}