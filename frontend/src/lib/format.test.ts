import { describe, it, expect } from 'vitest'
import { formatDuration, formatTokens, formatRelativeTime } from './format'

describe('formatDuration', () => {
  it('formats seconds', () => {
    expect(formatDuration(45)).toBe('45s')
  })

  it('formats minutes and seconds', () => {
    expect(formatDuration(125)).toBe('2m 5s')
  })

  it('formats hours and minutes', () => {
    expect(formatDuration(3665)).toBe('1h 1m')
  })

  it('handles zero', () => {
    expect(formatDuration(0)).toBe('0s')
  })
})

describe('formatTokens', () => {
  it('formats small numbers as-is', () => {
    expect(formatTokens(500)).toBe('500')
  })

  it('formats thousands with k suffix', () => {
    expect(formatTokens(12500)).toBe('12.5k')
  })

  it('formats millions with M suffix', () => {
    expect(formatTokens(1_500_000)).toBe('1.50M')
  })
})

describe('formatRelativeTime', () => {
  it('formats future dates as "in X"', () => {
    const future = new Date(Date.now() + 60000).toISOString()
    expect(formatRelativeTime(future)).toMatch(/^in /)
  })

  it('formats past dates as "X ago"', () => {
    const past = new Date(Date.now() - 120000).toISOString()
    expect(formatRelativeTime(past)).toMatch(/ago$/)
  })
})
