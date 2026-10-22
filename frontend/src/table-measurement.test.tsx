// @vitest-environment jsdom
import { act, cleanup, render, waitFor } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { alertSchema } from './api'
import { AlertsTable } from './components/AlertsTable'

vi.mock('@wailsio/runtime', () => ({
  Call: { ByName: vi.fn().mockResolvedValue(undefined) },
  Events: { On: vi.fn() }, Clipboard: {}, Browser: {},
}))

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

it('measures wrapped virtual rows and repositions subsequent rows after resize', async () => {
  let rowHeight = 80
  const observers = new Set<TestObserver>()
  class TestObserver implements ResizeObserver {
    elements = new Set<Element>()
    constructor(readonly callback: ResizeObserverCallback) { observers.add(this) }
    observe(element: Element) { this.elements.add(element) }
    unobserve(element: Element) { this.elements.delete(element) }
    disconnect() { this.elements.clear(); observers.delete(this) }
    resize(element: Element) {
      if (!this.elements.has(element)) return
      const size = { blockSize: rowHeight, inlineSize: 420 }
      this.callback([{
        target: element,
        contentRect: new DOMRectReadOnly(0, 0, 420, rowHeight),
        borderBoxSize: [size], contentBoxSize: [size], devicePixelContentBoxSize: [size],
      }], this)
    }
  }
  vi.stubGlobal('ResizeObserver', TestObserver)
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(420)
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(300)
  vi.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockImplementation(function (this: HTMLElement) {
    return this.classList.contains('table-row') ? rowHeight : 300
  })
  const alerts = [2, 1].map((id) => alertSchema.parse({
    id, ruleId: 1, ruleName: 'listing', symbol: 'BTCUSDT', direction: 'down', changePercent: -3,
    price: 1, spreadPercent: 0, quoteVolume: 1, eventTimeMs: id, createdAtMs: id,
    sound: 'chime', color: '#41c977', volume: .5, terminals: [], terminalStatus: '', terminalError: '',
  }))
  const { container } = render(<AlertsTable alerts={alerts} rules={[]} hasMore={false} loadMore={vi.fn()} onError={vi.fn()} />)
  await waitFor(() => expect(container.querySelectorAll('.table-row')).toHaveLength(2))
  const rows = container.querySelectorAll<HTMLElement>('.table-row')
  await waitFor(() => expect(rows[1].style.transform).toBe('translateY(80px)'))
  const viewport = container.querySelector('.table-scroll')
  if (!viewport) throw new Error('missing viewport')
  act(() => {
    for (const observer of [...observers]) observer.resize(viewport)
  })
  await waitFor(() => expect(rows[1].style.transform).toBe('translateY(80px)'))
  rowHeight = 120
  act(() => {
    for (const observer of [...observers]) observer.resize(rows[0])
  })
  await waitFor(() => expect(rows[1].style.transform).toBe('translateY(120px)'))
})
