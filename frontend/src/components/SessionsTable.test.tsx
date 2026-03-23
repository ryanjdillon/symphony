import { describe, it, expect, vi } from 'vitest'
import { render, within } from '@/test/test-utils'
import userEvent from '@testing-library/user-event'
import { SessionsTable } from './SessionsTable'
import { mockState, emptyState } from '@/test/fixtures'

describe('SessionsTable', () => {
  it('renders session identifiers', () => {
    const { container } = render(<SessionsTable sessions={mockState.running} onSelect={vi.fn()} />)
    expect(within(container).getByText('SYM-1')).toBeInTheDocument()
    expect(within(container).getByText('SYM-2')).toBeInTheDocument()
  })

  it('shows empty message when no sessions', () => {
    const { container } = render(<SessionsTable sessions={emptyState.running} onSelect={vi.fn()} />)
    expect(within(container).getByText('No running sessions')).toBeInTheDocument()
  })

  it('calls onSelect when row is clicked', async () => {
    const user = userEvent.setup()
    const onSelect = vi.fn()
    const { container } = render(<SessionsTable sessions={mockState.running} onSelect={onSelect} />)
    await user.click(within(container).getByText('SYM-1'))
    expect(onSelect).toHaveBeenCalledWith('SYM-1')
  })

  it('displays formatted elapsed time', () => {
    const { container } = render(<SessionsTable sessions={mockState.running} onSelect={vi.fn()} />)
    expect(within(container).getByText('2m 5s')).toBeInTheDocument()
  })
})
