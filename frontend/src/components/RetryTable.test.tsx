import { describe, it, expect } from 'vitest'
import { render, within } from '@/test/test-utils'
import { RetryTable } from './RetryTable'
import { mockState, emptyState } from '@/test/fixtures'

describe('RetryTable', () => {
  it('renders retry entries', () => {
    const { container } = render(<RetryTable retries={mockState.retrying} />)
    expect(within(container).getByText('SYM-3')).toBeInTheDocument()
    expect(within(container).getByText('stalled')).toBeInTheDocument()
  })

  it('shows empty message when no retries', () => {
    const { container } = render(<RetryTable retries={emptyState.retrying} />)
    expect(within(container).getByText('No pending retries')).toBeInTheDocument()
  })

  it('shows attempt count', () => {
    const { container } = render(<RetryTable retries={mockState.retrying} />)
    expect(within(container).getByText('2')).toBeInTheDocument()
  })
})
