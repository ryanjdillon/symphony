import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useWebSocket } from './useWebSocket'

class MockWebSocket {
  static instances: MockWebSocket[] = []
  url: string
  readyState: number = 0
  onopen: ((event: Event) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  onerror: ((event: Event) => void) | null = null
  send = vi.fn()
  close = vi.fn()

  constructor(url: string) {
    this.url = url
    MockWebSocket.instances.push(this)
  }

  simulateOpen() {
    this.readyState = WebSocket.OPEN
    this.onopen?.(new Event('open'))
  }

  simulateMessage(data: unknown) {
    this.onmessage?.(new MessageEvent('message', { data: JSON.stringify(data) }))
  }

  static get CONNECTING() { return 0 }
  static get OPEN() { return 1 }
  static get CLOSING() { return 2 }
  static get CLOSED() { return 3 }
}

describe('useWebSocket', () => {
  beforeEach(() => {
    MockWebSocket.instances = []
    vi.stubGlobal('WebSocket', MockWebSocket)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('connects to the provided URL', () => {
    renderHook(() => useWebSocket('ws://localhost:8080/ws'))
    expect(MockWebSocket.instances).toHaveLength(1)
    expect(MockWebSocket.instances[0].url).toBe('ws://localhost:8080/ws')
  })

  it('sets connected to true on open', () => {
    const { result } = renderHook(() => useWebSocket('ws://localhost:8080/ws'))
    const ws = MockWebSocket.instances[0]

    act(() => ws.simulateOpen())

    expect(result.current.connected).toBe(true)
  })

  it('parses state_update messages', () => {
    const { result } = renderHook(() => useWebSocket('ws://localhost:8080/ws'))
    const ws = MockWebSocket.instances[0]

    act(() => ws.simulateOpen())

    const mockData = {
      type: 'state_update',
      data: {
        running: [],
        retrying: [],
        tokens: { input: 0, output: 0, total: 0 },
        runtime_s: 100,
      },
    }

    act(() => ws.simulateMessage(mockData))

    expect(result.current.state).toEqual(mockData.data)
  })

  it('ignores non-state_update messages', () => {
    const { result } = renderHook(() => useWebSocket('ws://localhost:8080/ws'))
    const ws = MockWebSocket.instances[0]

    act(() => ws.simulateOpen())
    act(() => ws.simulateMessage({ type: 'other' }))

    expect(result.current.state).toBeNull()
  })

  it('sends refresh via WebSocket', () => {
    const { result } = renderHook(() => useWebSocket('ws://localhost:8080/ws'))
    const ws = MockWebSocket.instances[0]

    act(() => ws.simulateOpen())
    act(() => result.current.sendRefresh())

    expect(ws.send).toHaveBeenCalledWith(JSON.stringify({ type: 'refresh' }))
  })
})
