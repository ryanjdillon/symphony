import type { RunningSession } from '@/types/api'
import { formatDuration } from '@/lib/format'

interface SessionsTableProps {
  sessions: RunningSession[]
  onSelect: (identifier: string) => void
}

export function SessionsTable({ sessions, onSelect }: SessionsTableProps) {
  if (sessions.length === 0) {
    return (
      <p className="py-8 text-center text-sm text-zinc-400 dark:text-zinc-500">
        No running sessions
      </p>
    )
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-left text-sm">
        <thead className="border-b border-zinc-200 text-xs uppercase text-zinc-500 dark:border-zinc-700 dark:text-zinc-400">
          <tr>
            <th className="px-4 py-3">Issue</th>
            <th className="px-4 py-3">State</th>
            <th className="px-4 py-3">Turns</th>
            <th className="px-4 py-3">Elapsed</th>
          </tr>
        </thead>
        <tbody>
          {sessions.map((s) => (
            <tr
              key={s.issue_id}
              onClick={() => onSelect(s.identifier)}
              className="cursor-pointer border-b border-zinc-100 hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-800/50"
            >
              <td className="px-4 py-3 font-medium text-zinc-900 dark:text-zinc-100">
                {s.identifier}
              </td>
              <td className="px-4 py-3 text-zinc-600 dark:text-zinc-300">
                {s.state}
              </td>
              <td className="px-4 py-3 text-zinc-600 dark:text-zinc-300">
                {s.turn_count}
              </td>
              <td className="px-4 py-3 text-zinc-600 dark:text-zinc-300">
                {formatDuration(s.elapsed_s)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
