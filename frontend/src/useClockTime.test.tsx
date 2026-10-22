// @vitest-environment jsdom
import { act, cleanup, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { useClockTime } from './useClockTime'

const api = vi.hoisted(() => ({ clockOffset: vi.fn() }))
vi.mock('./api', () => ({ desktop: api }))

beforeEach(() => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-10-21T12:04:54.250Z'))
  api.clockOffset.mockImplementation(async () => 200)
})
afterEach(() => { cleanup(); vi.useRealTimers(); vi.clearAllMocks() })

it('aligns ticks to exchange seconds and periodically refreshes synchronization', async () => {
  const { result, unmount } = renderHook(useClockTime)
  await act(async () => {})
  expect(result.current).toBe(Date.now() + 200)
  await act(async () => { await vi.advanceTimersByTimeAsync(550) })
  expect(new Date(result.current).toISOString()).toBe('2026-10-21T12:04:55.000Z')
  await act(async () => { await vi.advanceTimersByTimeAsync(61_000) })
  expect(api.clockOffset).toHaveBeenCalledTimes(6)
  unmount()
  expect(vi.getTimerCount()).toBe(0)
})

it('falls back to local time when synchronization fails', async () => {
  api.clockOffset.mockRejectedValue(new Error('offline'))
  const { result } = renderHook(useClockTime)
  await act(async () => {})
  expect(result.current).toBe(Date.now())
  await act(async () => { await vi.advanceTimersByTimeAsync(750) })
  expect(new Date(result.current).toISOString()).toBe('2026-10-21T12:04:55.000Z')
})

it('recovers from sleep or local clock changes without replaying missed ticks', async () => {
  const { result } = renderHook(useClockTime)
  await act(async () => {})
  vi.setSystemTime(new Date('2026-10-22T00:00:00.100Z'))
  await act(async () => { await vi.advanceTimersByTimeAsync(1000) })
  expect(result.current).toBeGreaterThanOrEqual(Date.parse('2026-10-22T00:00:00Z'))
  expect(api.clockOffset).toHaveBeenCalledTimes(6)
})

it('does not schedule work after an in-flight request resolves on a closed window', async () => {
  let resolve: (timestamp: number) => void = () => { throw new Error('request has not started') }
  api.clockOffset.mockImplementation(() => new Promise<number>((done) => { resolve = done }))
  const { unmount } = renderHook(useClockTime)
  unmount()
  await act(async () => { resolve(Date.now()) })
  expect(vi.getTimerCount()).toBe(0)
})

it('chooses the probe with the least delay and keeps the backend RTT compensation', async () => {
  let monotonic = 0
  const timing = vi.spyOn(performance, 'now').mockImplementation(() => monotonic)
  const samples = [{ delay: 200, offset: 900 }, { delay: 10, offset: 120 }, { delay: 100, offset: 700 }]
  let index = 0
  api.clockOffset.mockImplementation(async () => {
    const sample = samples[index++]
    monotonic += sample.delay
    vi.setSystemTime(Date.now() + sample.delay)
    return sample.offset
  })
  const { result } = renderHook(useClockTime)
  await act(async () => {})
  expect(result.current).toBe(Date.now() + 120)
  timing.mockRestore()
})
