// API types matching the Go backend structures

export interface APIResponse<T> {
  ok: boolean
  data?: T
  error?: { code: string; message: string }
}

export interface AgentRecord {
  agent_id: string
  name: string
  version: string
  os: string
  arch: string
  mode: string
  capabilities?: string[]
  connected_at: string
  last_heartbeat: string
  status: string // active, inactive, stale, error
  uptime_seconds: number
  last_error?: string
  error_count: number
  health_score: number
  resource_usage?: {
    cpu_percent: number
    memory_mb: number
    disk_free_mb: number
  }
}

export interface HealthInfo {
  status: string
  total_agents: number
  active_agents: number
  stale_agents: number
  uptime_seconds: number
}

export interface BuildConfig {
  id?: string
  name: string
  os: string
  arch: string
  capabilities: string[]
  server_url: string
  token: string
  permissions: string
  sandbox_dir?: string
  disguise?: DisguiseConfig
  autostart: boolean
  backoff_min?: string
  backoff_max?: string
  max_retries?: number
  status?: string
  binary_path?: string
  created_at?: string
  completed_at?: string
  error?: string
}

export interface DisguiseConfig {
  enabled: boolean
  filename: string
  company: string
  description: string
  product_name: string
}

export interface Profile {
  id: string
  name: string
  os: string
  arch: string
  capabilities: string[]
  server_url: string
  permissions: string
  sandbox_dir?: string
  disguise?: DisguiseConfig
  autostart: boolean
  backoff_min?: string
  backoff_max?: string
  max_retries?: number
  created_at: string
}

export interface Task {
  id: string
  agent_id: string
  command_type: string
  params: unknown
  schedule: {
    type: string
    delay_seconds?: number
    interval_seconds?: number
    max_retries?: number
    retry_count?: number
  }
  status: string
  result?: unknown
  error?: string
  created_at: string
  execute_at: string
  started_at?: string
  completed_at?: string
  operator_id?: string
}

export interface Operator {
  id: string
  name: string
  role: string
  created_at: string
  last_seen?: string
}

export interface EnrollmentToken {
  token: string
  agent_name: string
  created_at: string
  expires_at: string
  used: boolean
}

export interface AuditEntry {
  id: string
  timestamp: string
  agent_id: string
  operator_id: string
  action: string
  command_type?: string
  result?: string
  error?: string
}

export interface RevokedAgent {
  agent_id: string
  revoked_at: string
  reason: string
}