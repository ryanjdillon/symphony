import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, waitFor, within } from '@/test/test-utils'
import { Dashboard } from './Dashboard'
import { mockState } from '@/test/fixtures'

vi.mock('@/hooks/useOrchestratorState', () => ({
  useOrchestratorState: () => ({
    state: mockState,
    connected: true,
    loading: false,
    error: null,
    sendRefresh: vi.fn(),
  }),
}))

vi.mock('@/lib/api', () => ({
  triggerRefresh: vi.fn().mockResolvedValue(undefined),
  fetchIssue: vi.fn(),
}))

describe('Dashboard', () => {
  let container: HTMLElement

  beforeEach(() => {
    vi.clearAllMocks()
    const result = render(<Dashboard />)
    container = result.container
  })

  it('renders the title', () => {
    expect(within(container).getByText('Symphony')).toBeInTheDocument()
  })

  it('shows connected status badge', () => {
    expect(within(container).getByText('Live')).toBeInTheDocument()
  })

  it('renders metric cards', () => {
    expect(within(container).getByText('Tokens')).toBeInTheDocument()
    expect(within(container).getByText('Runtime')).toBeInTheDocument()
  })

  it('renders running sessions section', () => {
    expect(within(container).getByText('Running Sessions')).toBeInTheDocument()
    expect(within(container).getByText('SYM-1')).toBeInTheDocument()
  })

  it('renders retry queue section', () => {
    expect(within(container).getByText('Retry Queue')).toBeInTheDocument()
    expect(within(container).getByText('SYM-3')).toBeInTheDocument()
  })

  it('has a refresh button', () => {
    expect(within(container).getByRole('button', { name: 'Refresh' })).toBeInTheDocument()
  })
})

describe('Dashboard navigation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('navigates to issue detail when row clicked', async () => {
    const { fetchIssue } = await import('@/lib/api')
    vi.mocked(fetchIssue).mockResolvedValue({
      issue_id: 'id-1',
      identifier: 'SYM-1',
      state: 'In Progress',
      session_id: 'sess-abc',
      turn_count: 3,
      elapsed_s: 125,
      started_at: '2026-03-16T10:00:00Z',
      last_event: '2026-03-16T10:02:05Z',
      tokens: { input: 10000, output: 5000, total: 15000 },
    })

    const { container } = render(<Dashboard />)
    within(container).getByText('SYM-1').closest('tr')!.click()

    await waitFor(() => {
      expect(within(container).getByText(/Back to dashboard/)).toBeInTheDocument()
    })
  })
})
