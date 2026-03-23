import { describe, it, expect } from 'vitest'
import { render, screen } from '@/test/test-utils'
import { MetricCard } from './MetricCard'

describe('MetricCard', () => {
  it('renders label and value', () => {
    render(<MetricCard label="Running" value={5} />)
    expect(screen.getByText('Running')).toBeInTheDocument()
    expect(screen.getByText('5')).toBeInTheDocument()
  })

  it('renders sublabel when provided', () => {
    render(<MetricCard label="Tokens" value="62k" sublabel="50k in / 12k out" />)
    expect(screen.getByText('50k in / 12k out')).toBeInTheDocument()
  })

  it('does not render sublabel when absent', () => {
    const { container } = render(<MetricCard label="Running" value={0} />)
    expect(container.querySelectorAll('p')).toHaveLength(2) // label + value only
  })
})
