export interface RunningSession {
  issue_id: string
  identifier: string
  state: string
  session_id: string
  turn_count: number
  elapsed_s: number
}

export interface RetryEntry {
  issue_id: string
  identifier: string
  attempt: number
  due_at: string
  error: string
}

export interface TokenUsage {
  input: number
  output: number
  total: number
}

export interface OrchestratorState {
  running: RunningSession[]
  retrying: RetryEntry[]
  tokens: TokenUsage
  runtime_s: number
}

export interface IssueDetail {
  issue_id: string
  identifier: string
  state: string
  session_id: string
  turn_count: number
  elapsed_s: number
  started_at: string
  last_event: string
  tokens: TokenUsage
}

export interface WsMessage {
  type: string
  data?: OrchestratorState
}
