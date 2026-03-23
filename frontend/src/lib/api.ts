import type { IssueDetail, OrchestratorState } from '@/types/api'

const BASE = ''

export async function fetchState(): Promise<OrchestratorState> {
  const res = await fetch(`${BASE}/api/v1/state`)
  if (!res.ok) throw new Error(`Failed to fetch state: ${res.status}`)
  return res.json()
}

export async function fetchIssue(identifier: string): Promise<IssueDetail> {
  const res = await fetch(`${BASE}/api/v1/${encodeURIComponent(identifier)}`)
  if (!res.ok) throw new Error(`Failed to fetch issue: ${res.status}`)
  return res.json()
}

export async function triggerRefresh(): Promise<void> {
  const res = await fetch(`${BASE}/api/v1/refresh`, { method: 'POST' })
  if (!res.ok) throw new Error(`Failed to trigger refresh: ${res.status}`)
}
