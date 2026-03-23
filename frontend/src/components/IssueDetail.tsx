import { useEffect, useState } from 'react'
import type { IssueDetail as IssueDetailType } from '@/types/api'
import { fetchIssue } from '@/lib/api'
import { formatDuration, formatTokens } from '@/lib/format'

interface IssueDetailProps {
  identifier: string
  onBack: () => void
}

export function IssueDetail({ identifier, onBack }: IssueDetailProps) {
  const [issue, setIssue] = useState<IssueDetailType | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    fetchIssue(identifier)
      .then(setIssue)
      .catch((err) => setError(err.message))
  }, [identifier])

  return (
    <div>
      <button
        onClick={onBack}
        className="mb-4 text-sm text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
      >
        &larr; Back to dashboard
      </button>

      <h2 className="text-xl font-semibold text-zinc-900 dark:text-zinc-100">
        {identifier}
      </h2>

      {error && (
        <p className="mt-2 text-sm text-red-500">
          {error}
        </p>
      )}

      {issue && (
        <div className="mt-4 space-y-3">
          <dl className="grid grid-cols-2 gap-x-8 gap-y-2 text-sm">
            <dt className="text-zinc-500 dark:text-zinc-400">State</dt>
            <dd className="text-zinc-900 dark:text-zinc-100">{issue.state}</dd>

            <dt className="text-zinc-500 dark:text-zinc-400">Session</dt>
            <dd className="font-mono text-xs text-zinc-700 dark:text-zinc-300">
              {issue.session_id}
            </dd>

            <dt className="text-zinc-500 dark:text-zinc-400">Turns</dt>
            <dd className="text-zinc-900 dark:text-zinc-100">{issue.turn_count}</dd>

            <dt className="text-zinc-500 dark:text-zinc-400">Elapsed</dt>
            <dd className="text-zinc-900 dark:text-zinc-100">
              {formatDuration(issue.elapsed_s)}
            </dd>

            <dt className="text-zinc-500 dark:text-zinc-400">Started</dt>
            <dd className="text-zinc-900 dark:text-zinc-100">
              {new Date(issue.started_at).toLocaleString()}
            </dd>

            <dt className="text-zinc-500 dark:text-zinc-400">Last Event</dt>
            <dd className="text-zinc-900 dark:text-zinc-100">
              {new Date(issue.last_event).toLocaleString()}
            </dd>

            <dt className="text-zinc-500 dark:text-zinc-400">Tokens (in / out / total)</dt>
            <dd className="text-zinc-900 dark:text-zinc-100">
              {formatTokens(issue.tokens.input)} / {formatTokens(issue.tokens.output)} / {formatTokens(issue.tokens.total)}
            </dd>
          </dl>
        </div>
      )}
    </div>
  )
}
