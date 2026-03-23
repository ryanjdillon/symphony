import { useEffect, useState } from 'react'
import type { OrchestratorState } from '@/types/api'
import { fetchState } from '@/lib/api'
import { useWebSocket } from './useWebSocket'

function getWsUrl(): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}/ws`
}

export function useOrchestratorState() {
  const { state: wsState, connected, sendRefresh } = useWebSocket(getWsUrl())
  const [initialState, setInitialState] = useState<OrchestratorState | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchState()
      .then((data) => {
        setInitialState(data)
        setError(null)
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))
  }, [])

  const state = wsState ?? initialState

  return { state, connected, loading, error, sendRefresh }
}
