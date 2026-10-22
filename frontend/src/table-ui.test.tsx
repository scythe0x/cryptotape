// @vitest-environment jsdom
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { alertSchema, defaultTableColumns, settingsSchema, type Alert } from './api'
import { applyLanguage } from './i18n'
import { AlertsTable, sortAlerts } from './components/AlertsTable'
import { ActiveItems } from './components/ActiveItems'
import { ListingSummary } from './components/ListingSummary'
import { TableColumns } from './components/TableColumns'

const backend = vi.hoisted(() => ({ call: vi.fn().mockResolvedValue(undefined), measure: vi.fn() }))
vi.mock('@wailsio/runtime', () => ({ Call: { ByName: backend.call }, Events: { On: vi.fn() }, Clipboard: {}, Browser: {} }))
vi.mock('@tanstack/react-virtual', () => ({
  useVirtualizer: ({ count }: { count: number }) => ({
    getVirtualItems: () => Array.from({ length: count }, (_, index) => ({ index, start: index * 40 })),
    getTotalSize: () => count * 40,
    measureElement: backend.measure,
    measure: backend.measure,
    scrollToOffset: vi.fn(),
  }),
}))

function alert(id: number, createdAtMs = id): Alert {
  return alertSchema.parse({ id, ruleId: 1, ruleName: 'soft move', symbol: 'BTCUSDT', direction: 'down',
    changePercent: -3.01, price: 1, spreadPercent: 0, quoteVolume: 1, eventTimeMs: createdAtMs,
    createdAtMs, sound: 'chime', color: '#41c977', volume: .5, terminals: [], terminalStatus: '', terminalError: '' })
}
beforeEach(() => {
  applyLanguage('en')
  backend.call.mockClear()
  backend.measure.mockClear()
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} })
})
afterEach(() => { cleanup(); vi.unstubAllGlobals() })

it('sorts by timestamp then id without mutating the feed', () => {
  const values = [alert(10, 1), alert(1, 3), alert(2, 3)]
  expect(sortAlerts(values).map((item) => item.id)).toEqual([2, 1, 10])
  expect(values.map((item) => item.id)).toEqual([10, 1, 2])
})

it('combines the badge and wrapping event without a column heading row', () => {
  const { container } = render(<AlertsTable alerts={[alert(1)]} rules={[]} hasMore={false} loadMore={vi.fn()} onError={vi.fn()} />)
  const event = container.querySelector('.feed-cell-event')
  expect(event?.textContent).toBe('soft move-3.01%')
  expect(container.querySelector('.table-header')).toBeNull()
  expect(container.querySelector('.table-row')?.getAttribute('data-index')).toBe('0')
  expect(backend.measure).toHaveBeenCalled()
  expect(screen.getByRole('region', { name: 'alert history' }).getAttribute('tabindex')).toBe('0')
})

it('shows a listing event followed by its exchange', () => {
  render(<ListingSummary alert={{ ...alert(1), kind: 'listing', direction: '', listing: {
    exchange: 'mexc', asset: 'BTC', marketType: 'futures', markets: ['BTCUSDT'],
    url: 'https://example.com/listing', detectedTimeUs: 1, sentTimeUs: 1,
  } }} />)
  expect(screen.getByText('Futures listing MEXC')).toBeTruthy()
})

it('respects ordered columns, including an entirely hidden table', () => {
  const props = { alerts: [alert(1)], rules: [], hasMore: false, loadMore: vi.fn(), onError: vi.fn() }
  const { container, rerender } = render(<AlertsTable {...props} columns={['event', 'time']} />)
  expect([...container.querySelectorAll('.table-row > .table-cell')].map((node) => node.className)).toEqual(['table-cell feed-cell-event', 'table-cell feed-cell-time'])
  expect(container.querySelector('.feed-cell-ticker')).toBeNull()
  rerender(<AlertsTable {...props} columns={[]} />)
  expect(container.querySelector('.table-row')).toBeNull()
  expect(container.querySelector('.table-header')).toBeNull()
})

it('requests older alerts near the end of the scroll viewport', () => {
  const loadMore = vi.fn()
  const { container } = render(<AlertsTable alerts={[alert(1)]} rules={[]} hasMore loadMore={loadMore} onError={vi.fn()} columns={defaultTableColumns} />)
  const viewport = container.querySelector('.table-scroll')
  if (!viewport) throw new Error('missing viewport')
  fireEvent.scroll(viewport)
  expect(loadMore).toHaveBeenCalledOnce()
})

it('exposes both drop zones even when all items are inactive', () => {
  const onChange = vi.fn()
  const items = [{ id: 'one', label: 'first', content: 'first' }, { id: 'two', label: 'second', content: 'second' }]
  render(<ActiveItems items={items} value={[]} onChange={onChange} activeLabel="active" inactiveLabel="inactive" />)
  expect(within(screen.getByRole('group', { name: 'active' })).queryAllByRole('button')).toHaveLength(0)
  expect(within(screen.getByRole('group', { name: 'inactive' })).getAllByRole('button')).toHaveLength(2)
  expect(screen.getByRole('button', { name: 'first' }).getAttribute('tabindex')).not.toBe('-1')
  expect(onChange).not.toHaveBeenCalled()
})

it('supports keyboard reordering and moving into an empty zone', () => {
  const onChange = vi.fn()
  const items = [{ id: 'one', label: 'first', content: 'first' }, { id: 'two', label: 'second', content: 'second' }]
  const { rerender } = render(<ActiveItems items={items} value={['one', 'two']} onChange={onChange} activeLabel="active" inactiveLabel="inactive" />)
  fireEvent.keyDown(screen.getByRole('button', { name: 'second' }), { key: 'ArrowLeft' })
  expect(onChange).toHaveBeenLastCalledWith(['two', 'one'])
  rerender(<ActiveItems items={items} value={['two', 'one']} onChange={onChange} activeLabel="active" inactiveLabel="inactive" />)
  fireEvent.keyDown(screen.getByRole('button', { name: 'second' }), { key: 'ArrowDown' })
  expect(onChange).toHaveBeenLastCalledWith(['one'])
  rerender(<ActiveItems items={items} value={[]} onChange={onChange} activeLabel="active" inactiveLabel="inactive" />)
  fireEvent.keyDown(screen.getByRole('button', { name: 'first' }), { key: 'ArrowUp' })
  expect(onChange).toHaveBeenLastCalledWith(['one'])
})


it('saves only the reordered columns through their dedicated endpoint', async () => {
  const settings = settingsSchema.parse({ language: 'en', proxies: [], activeProxyId: '', monitorId: '', tigerPort: 7819, tigerLinkGroup: 'A' })
  render(<TableColumns settings={settings} onError={vi.fn()} />)
  fireEvent.keyDown(screen.getByRole('button', { name: 'ticker' }), { key: 'ArrowLeft' })
  await waitFor(() => expect(backend.call).toHaveBeenCalledWith('main.DesktopService.SetTableColumns', ['ticker', 'time', 'event', 'terminals']))
  expect(backend.call).not.toHaveBeenCalledWith('main.DesktopService.UpdateSettings', expect.anything())
})

it('reports a failed column save without hiding the previous selection', async () => {
  const settings = settingsSchema.parse({ language: 'en', proxies: [], activeProxyId: '', monitorId: '', tigerPort: 7819, tigerLinkGroup: 'A' })
  const onError = vi.fn()
  backend.call.mockRejectedValueOnce(new Error('disk full'))
  render(<TableColumns settings={settings} onError={onError} />)
  fireEvent.keyDown(screen.getByRole('button', { name: 'ticker' }), { key: 'ArrowDown' })
  await waitFor(() => expect(onError).toHaveBeenCalledWith('Error: disk full'))
  expect(within(screen.getByRole('group', { name: 'active columns' })).getAllByRole('button')).toHaveLength(4)
})


it('shows the copy confirmation only on the clicked alert when tickers repeat', async () => {
  const copy = vi.spyOn((await import('./api')).desktop, 'copySymbol').mockResolvedValue(undefined)
  const { container } = render(<AlertsTable alerts={[alert(2), alert(1)]} rules={[]} hasMore={false} loadMore={vi.fn()} onError={vi.fn()} />)
  const buttons = screen.getAllByTitle('copy BTCUSDT')
  fireEvent.click(buttons[0])
  await waitFor(() => expect(container.querySelectorAll('[aria-label="copied"]')).toHaveLength(1))
  expect(buttons[0].querySelector('[aria-label="copied"]')).toBeTruthy()
  expect(buttons[1].querySelector('[aria-label="copied"]')).toBeNull()
  fireEvent.click(buttons[1])
  await waitFor(() => expect(buttons[1].querySelector('[aria-label="copied"]')).toBeTruthy())
  expect(buttons[0].querySelector('[aria-label="copied"]')).toBeNull()
  expect(copy).toHaveBeenCalledTimes(2)
  copy.mockRestore()
})
