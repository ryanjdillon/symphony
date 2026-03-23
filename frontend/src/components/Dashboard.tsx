import { useState } from 'react'
import { useOrchestratorState } from '@/hooks/useOrchestratorState'
import { triggerRefresh } from '@/lib/api'
import { formatDuration, formatTokens } from '@/lib/format'
import { StatusBadge } from './StatusBadge'
import { MetricCard } from './MetricCard'
import { SessionsTable } from './SessionsTable'
import { RetryTable } from './RetryTable'
import { IssueDetail } from './IssueDetail'

export function Dashboard() {
  const { state, connected, loading, error } = useOrchestratorState()
  const [selectedIssue, setSelectedIssue] = useState<string | null>(null)
  const [refreshing, setRefreshing] = useState(false)

  const handleRefresh = async () => {
    setRefreshing(true)
    try {
      await triggerRefresh()
    } finally {
      setTimeout(() => setRefreshing(false), 1000)
    }
  }

  if (selectedIssue) {
    return (
      <IssueDetail
        identifier={selectedIssue}
        onBack={() => setSelectedIssue(null)}
      />
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h1 className="text-2xl font-bold text-zinc-900 dark:text-zinc-100">
            Symphony
          </h1>
          <StatusBadge connected={connected} />
        </div>
        <button
          onClick={handleRefresh}
          disabled={refreshing}
          className="rounded-md bg-zinc-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-zinc-700 disabled:opacity-50 dark:bg-zinc-100 dark:text-zinc-900 dark:hover:bg-zinc-300"
        >
          {refreshing ? 'Refreshing...' : 'Refresh'}
        </button>
      </div>

      {loading && (
        <p className="text-sm text-zinc-500">Loading...</p>
      )}

      {error && (
        <p className="text-sm text-red-500">{error}</p>
      )}

      {state && (
        <>
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
            <MetricCard
              label="Running"
              value={state.running.length}
            />
            <MetricCard
              label="Retrying"
              value={state.retrying.length}
            />
            <MetricCard
              label="Tokens"
              value={formatTokens(state.tokens.total)}
              sublabel={`${formatTokens(state.tokens.input)} in / ${formatTokens(state.tokens.output)} out`}
            />
            <MetricCard
              label="Runtime"
              value={formatDuration(state.runtime_s)}
            />
          </div>

          <div className="rounded-lg border border-zinc-200 bg-white dark:border-zinc-700 dark:bg-zinc-800">
            <h2 className="border-b border-zinc-200 px-4 py-3 text-sm font-semibold text-zinc-900 dark:border-zinc-700 dark:text-zinc-100">
              Running Sessions
            </h2>
            <SessionsTable
              sessions={state.running}
              onSelect={setSelectedIssue}
            />
          </div>

          <div className="rounded-lg border border-zinc-200 bg-white dark:border-zinc-700 dark:bg-zinc-800">
            <h2 className="border-b border-zinc-200 px-4 py-3 text-sm font-semibold text-zinc-900 dark:border-zinc-700 dark:text-zinc-100">
              Retry Queue
            </h2>
            <RetryTable retries={state.retrying} />
          </div>
        </>
      )}
    </div>
  )
}
