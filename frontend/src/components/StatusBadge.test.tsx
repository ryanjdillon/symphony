import { describe, it, expect } from 'vitest'
import { render, screen } from '@/test/test-utils'
import { StatusBadge } from './StatusBadge'

describe('StatusBadge', () => {
  it('shows "Live" when connected', () => {
    render(<StatusBadge connected={true} />)
    expect(screen.getByText('Live')).toBeInTheDocument()
  })

  it('shows "Disconnected" when not connected', () => {
    render(<StatusBadge connected={false} />)
    expect(screen.getByText('Disconnected')).toBeInTheDocument()
  })
})
