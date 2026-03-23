interface MetricCardProps {
  label: string
  value: string | number
  sublabel?: string
}

export function MetricCard({ label, value, sublabel }: MetricCardProps) {
  return (
    <div className="rounded-lg border border-zinc-200 bg-white p-4 dark:border-zinc-700 dark:bg-zinc-800">
      <p className="text-sm text-zinc-500 dark:text-zinc-400">{label}</p>
      <p className="mt-1 text-2xl font-semibold text-zinc-900 dark:text-zinc-100">
        {value}
      </p>
      {sublabel && (
        <p className="mt-0.5 text-xs text-zinc-400 dark:text-zinc-500">
          {sublabel}
        </p>
      )}
    </div>
  )
}
