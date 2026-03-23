import type { OrchestratorState, IssueDetail } from '@/types/api'

export const mockState: OrchestratorState = {
  running: [
    {
      issue_id: 'id-1',
      identifier: 'SYM-1',
      state: 'In Progress',
      session_id: 'sess-abc',
      turn_count: 3,
      elapsed_s: 125,
    },
    {
      issue_id: 'id-2',
      identifier: 'SYM-2',
      state: 'Todo',
      session_id: 'sess-def',
      turn_count: 1,
      elapsed_s: 30,
    },
  ],
  retrying: [
    {
      issue_id: 'id-3',
      identifier: 'SYM-3',
      attempt: 2,
      due_at: new Date(Date.now() + 30000).toISOString(),
      error: 'stalled',
    },
  ],
  tokens: { input: 50000, output: 12000, total: 62000 },
  runtime_s: 3600,
}

export const emptyState: OrchestratorState = {
  running: [],
  retrying: [],
  tokens: { input: 0, output: 0, total: 0 },
  runtime_s: 0,
}

export const mockIssueDetail: IssueDetail = {
  issue_id: 'id-1',
  identifier: 'SYM-1',
  state: 'In Progress',
  session_id: 'sess-abc',
  turn_count: 3,
  elapsed_s: 125,
  started_at: '2026-03-16T10:00:00Z',
  last_event: '2026-03-16T10:02:05Z',
  tokens: { input: 10000, output: 5000, total: 15000 },
}
