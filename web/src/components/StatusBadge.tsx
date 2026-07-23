export function StatusBadge({ status }: { status: string }) {
  const cls = status === 'active' || status === 'completed' || status === 'ok' ? 'badge-green'
    : status === 'stale' || status === 'pending' || status === 'queued' || status === 'building' ? 'badge-yellow'
    : status === 'error' || status === 'failed' || status === 'cancelled' ? 'badge-red'
    : status === 'running' ? 'badge-blue'
    : 'badge-gray'
  return <span className={`badge ${cls}`}>{status}</span>
}