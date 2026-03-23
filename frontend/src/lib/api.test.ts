import { describe, it, expect, vi, beforeEach } from 'vitest'
import { fetchState, fetchIssue, triggerRefresh } from './api'
import { mockState, mockIssueDetail } from '@/test/fixtures'

describe('API client', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  describe('fetchState', () => {
    it('fetches and returns orchestrator state', async () => {
      vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(mockState),
      }))

      const result = await fetchState()
      expect(result).toEqual(mockState)
      expect(fetch).toHaveBeenCalledWith('/api/v1/state')
    })

    it('throws on non-ok response', async () => {
      vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
        ok: false,
        status: 500,
      }))

      await expect(fetchState()).rejects.toThrow('Failed to fetch state: 500')
    })
  })

  describe('fetchIssue', () => {
    it('fetches issue detail by identifier', async () => {
      vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(mockIssueDetail),
      }))

      const result = await fetchIssue('SYM-1')
      expect(result).toEqual(mockIssueDetail)
      expect(fetch).toHaveBeenCalledWith('/api/v1/SYM-1')
    })

    it('throws on 404', async () => {
      vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
        ok: false,
        status: 404,
      }))

      await expect(fetchIssue('SYM-999')).rejects.toThrow('Failed to fetch issue: 404')
    })
  })

  describe('triggerRefresh', () => {
    it('posts to refresh endpoint', async () => {
      vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true }))

      await triggerRefresh()
      expect(fetch).toHaveBeenCalledWith('/api/v1/refresh', { method: 'POST' })
    })

    it('throws on failure', async () => {
      vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
        ok: false,
        status: 500,
      }))

      await expect(triggerRefresh()).rejects.toThrow('Failed to trigger refresh: 500')
    })
  })
})
