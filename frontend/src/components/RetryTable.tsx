import type { RetryEntry } from '@/types/api'
import { formatRelativeTime } from '@/lib/format'

interface RetryTableProps {
  retries: RetryEntry[]
}

export function RetryTable({ retries }: RetryTableProps) {
  if (retries.length === 0) {
    return (
      <p className="py-8 text-center text-sm text-zinc-400 dark:text-zinc-500">
        No pending retries
      </p>
    )
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-left text-sm">
        <thead className="border-b border-zinc-200 text-xs uppercase text-zinc-500 dark:border-zinc-700 dark:text-zinc-400">
          <tr>
            <th className="px-4 py-3">Issue</th>
            <th className="px-4 py-3">Attempt</th>
            <th className="px-4 py-3">Due</th>
            <th className="px-4 py-3">Error</th>
          </tr>
        </thead>
        <tbody>
          {retries.map((r) => (
            <tr
              key={r.issue_id}
              className="border-b border-zinc-100 dark:border-zinc-800"
            >
              <td className="px-4 py-3 font-medium text-zinc-900 dark:text-zinc-100">
                {r.identifier}
              </td>
              <td className="px-4 py-3 text-zinc-600 dark:text-zinc-300">
                {r.attempt}
              </td>
              <td className="px-4 py-3 text-zinc-600 dark:text-zinc-300">
                {formatRelativeTime(r.due_at)}
              </td>
              <td className="max-w-xs truncate px-4 py-3 text-zinc-500 dark:text-zinc-400">
                {r.error}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
