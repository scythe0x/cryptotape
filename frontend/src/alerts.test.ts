import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Alert, AlertUpdate } from './api'
import { events } from './api'
import { mergeAlerts, notificationCards } from './alerts'

const listeners = vi.hoisted(() => new Map<string, (event: { data: unknown }) => void>())
vi.mock('@wailsio/runtime', () => ({
  Call: {},
  Events: {
    On(name: string, handle: (event: { data: unknown }) => void) {
      listeners.set(name, handle)
      return () => listeners.delete(name)
    },
  },
}))

function alert(id: number, status = 'pending'): Alert {
  return {
    bannerSeconds: -1,
  openExchanges: [],
    receivedAtMs: 1, detectedAtMs: 1, writeStartedAtMs: 1,
    id, ruleId: 1, ruleName: 'knife', symbol: `COIN${id}USDT`, direction: 'down',
    changePercent: -10, price: 90, spreadPercent: .01, quoteVolume: 100_000_000,
    eventTimeMs: 1, createdAtMs: 1, sound: 'chime', color: '#a78bfa', volume: 1, terminals: ['vataga'],
    terminalStatus: status, terminalError: '',
  }
}

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  listeners.clear()
})

describe('market crash presentation', () => {
  it.each([1, 3, 5, 10, 15, 1500])('keeps the newest five of %d notifications', (count) => {
    const incoming = Array.from({ length: count }, (_, index) => alert(index + 1))
    const cards = notificationCards([], incoming)
    expect(cards.map((card) => card.id)).toEqual(incoming.slice(-5).map((card) => card.id))
    expect(mergeAlerts([], incoming)).toHaveLength(count)
  })

  it('handles out-of-order and duplicate notification events', () => {
    expect(notificationCards([alert(20), alert(21)], [alert(19), alert(21), alert(22)]).map((card) => card.id))
      .toEqual([19, 20, 21, 22])
  })

  it.each(['pending', 'skipped'])('keeps terminal results when delayed %s events arrive', (status) => {
    const recovered = mergeAlerts([alert(1)], [alert(1, 'ok')])
    expect(mergeAlerts(recovered, [alert(1, status)])[0].terminalStatus).toBe('ok')
    expect(mergeAlerts([], [alert(1)], [{ id: 1, terminalStatus: 'error', terminalError: 'offline' }])[0].terminalError)
      .toBe('offline')
  })

  it('delivers 1500 alerts and terminal updates in one frame without loss', () => {
    vi.useFakeTimers()
    vi.stubGlobal('window', {
      requestAnimationFrame: (callback: FrameRequestCallback) => setTimeout(() => callback(16), 16),
      cancelAnimationFrame: clearTimeout,
      setTimeout, clearTimeout,
    })
    const batches: { alerts: Alert[]; updates: AlertUpdate[] }[] = []
    const stop = events.alerts((alerts, updates) => batches.push({ alerts, updates }))
    for (let id = 1; id <= 1500; id++) {
      listeners.get('alert:new')?.({ data: alert(id) })
      listeners.get('alert:update')?.({ data: { id, terminalStatus: 'ok', terminalError: '' } })
    }
    expect(batches).toHaveLength(0)
    vi.advanceTimersByTime(16)
    expect(batches).toHaveLength(1)
    const merged = mergeAlerts([], batches[0].alerts, batches[0].updates)
    expect(merged).toHaveLength(1500)
    expect(merged.every((item) => item.terminalStatus === 'ok')).toBe(true)
    vi.advanceTimersByTime(100)
    expect(batches).toHaveLength(1)
    stop()
    expect(listeners.size).toBe(0)
  })
})

it('recovers terminal results flushed before the native window publishes the alert', () => {
  vi.useFakeTimers()
  vi.stubGlobal('window', {
    requestAnimationFrame: (callback: () => void) => setTimeout(callback, 16),
    cancelAnimationFrame: clearTimeout,
    setTimeout,
    clearTimeout,
  })
  let current: Alert[] = []
  const stop = events.alerts((incoming, updates) => { current = mergeAlerts(current, incoming, updates) })
  listeners.get('alert:update')?.({ data: { id: 77, terminalStatus: 'ok', terminalError: '' } })
  vi.advanceTimersByTime(60)
  expect(current).toHaveLength(0)
  listeners.get('alert:new')?.({ data: alert(77) })
  vi.advanceTimersByTime(60)
  expect(current[0].terminalStatus).toBe('ok')
  stop()
})
