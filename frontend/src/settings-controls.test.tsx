// @vitest-environment jsdom
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { applyLanguage } from './i18n'
import { desktop, defaultOpenExchanges, type ListingSource, type Rule } from './api'
import { ListingSourceSettings } from './components/ListingSourceSettings'
import { RuleDialog } from './components/RuleDialog'
import { RulesSettings } from './components/RulesSettings'
import { Select } from './components/Select'

vi.mock('@wailsio/runtime', () => ({
  Call: { ByName: vi.fn() }, Clipboard: { SetText: vi.fn() }, Browser: { OpenURL: vi.fn() },
  Events: { On: () => () => {} },
}))
const rule: Rule = {
  id: 1, name: 'move', kind: 'price_move', percent: 3, windowSeconds: 60,
  color: '#41c977', volume: 0.5, minQuoteVolume: 80e6,
  terminals: ['vataga', 'tiger'], sound: 'chime', bannerSeconds: 15, openExchanges: [],
}
const source = (keyConfigured: boolean): ListingSource => ({
  endpoint: 'wss://ws.newlistings.pro/v2/full', keyConfigured,
  health: { state: 'connected', code: '', plan: '', delayMs: 0, received: 0,
    rejected: 0, unmatched: 0, lastEventAtMs: 0, providerWaitMs: 0, deliveryMs: 0 },
})
beforeEach(() => {
  applyLanguage('en')
  Element.prototype.scrollIntoView = vi.fn()
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} })
})
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.unstubAllGlobals() })

it('shows saved key, masks replacement and removes status after clearing', async () => {
  const load = vi.spyOn(desktop, 'listingSource').mockResolvedValue(source(true))
  const save = vi.spyOn(desktop, 'saveListingSource').mockResolvedValue(undefined)
  const clear = vi.spyOn(desktop, 'clearListingKey').mockResolvedValue(undefined)
  render(<ListingSourceSettings />)
  await screen.findByText('key saved')
  const input = screen.getByLabelText('API key')
  expect(input.getAttribute('type')).toBe('password')
  expect(input.getAttribute('placeholder')).toBe('*'.repeat(50))
  fireEvent.change(input, { target: { value: 'secret-value' } })
  fireEvent.click(screen.getByRole('button', { name: 'save' }))
  await waitFor(() => expect(save).toHaveBeenCalledWith('wss://ws.newlistings.pro/v2/full', 'secret-value'))
  await waitFor(() => expect(input.getAttribute('value')).toBe(''))
  load.mockResolvedValue(source(false))
  fireEvent.click(screen.getByRole('button', { name: 'remove key' }))
  await waitFor(() => expect(clear).toHaveBeenCalledOnce())
  await waitFor(() => expect(screen.queryByText('key saved')).toBeNull())
  expect(input.getAttribute('placeholder')).toBeNull()
})
it('does not submit placeholder as a replacement key', async () => {
  vi.spyOn(desktop, 'listingSource').mockResolvedValue(source(true))
  const save = vi.spyOn(desktop, 'saveListingSource').mockResolvedValue(undefined)
  render(<ListingSourceSettings />)
  await screen.findByText('key saved')
  fireEvent.click(screen.getByRole('button', { name: 'save' }))
  await waitFor(() => expect(save).toHaveBeenCalledWith('wss://ws.newlistings.pro/v2/full', ''))
})
it('uses terminal icons without nested buttons or auto text', () => {
  render(<RulesSettings rules={[rule]} sounds={[]} onSaved={() => {}} />)
  const card = screen.getByRole('button', { name: /move/ })
  expect(card.querySelectorAll('button').length).toBe(0)
  expect(card.querySelectorAll('img').length).toBe(2)
  expect(card.textContent).not.toContain('auto')
})
it('preserves disabled exchanges when changing alert type', async () => {
  const save = vi.spyOn(desktop, 'saveRule').mockResolvedValue(rule)
  render(<RuleDialog open onOpenChange={() => {}} rule={rule} sounds={[]} onSaved={() => {}} />)
  expect(screen.getByText('disabled exchanges')).toBeTruthy()
  fireEvent.keyDown(screen.getByRole('combobox', { name: 'alert type' }), { key: 'ArrowDown' })
  fireEvent.click(await screen.findByRole('option', { name: 'listing' }))
  fireEvent.click(screen.getByRole('button', { name: 'save alert' }))
  await waitFor(() => expect(save).toHaveBeenCalledWith(expect.objectContaining({ kind: 'listing', openExchanges: [] })))
})
it('new price alerts enable all supported exchanges', async () => {
  const save = vi.spyOn(desktop, 'saveRule').mockResolvedValue(rule)
  render(<RuleDialog open onOpenChange={() => {}} rule={null} sounds={[]} onSaved={() => {}} />)
  fireEvent.click(screen.getByRole('button', { name: 'save alert' }))
  await waitFor(() => expect(save).toHaveBeenCalledWith(expect.objectContaining({ openExchanges: defaultOpenExchanges })))
})
it('custom select supports empty values and keyboard opening', async () => {
  const change = vi.fn()
  render(<Select aria-label="monitor" value="other" onValueChange={change} options={[{ value: '', label: 'Primary' }, { value: 'other', label: 'Other' }]} />)
  fireEvent.keyDown(screen.getByRole('combobox'), { key: 'ArrowDown' })
  fireEvent.click(await screen.findByRole('option', { name: 'Primary' }))
  expect(change).toHaveBeenCalledWith('')
  expect(document.querySelector('select')).toBeNull()
})

it('composes select and scroll viewport without conflicting overflow styles', async () => {
  const errors = vi.spyOn(console, 'error')
  const options = Array.from({ length: 40 }, (_, index) => ({ value: String(index), label: `option ${index}` }))
  const { rerender } = render(<Select aria-label="scrolling options" value="0" onValueChange={() => {}} options={options} />)
  fireEvent.keyDown(screen.getByRole('combobox'), { key: 'ArrowDown' })
  const option = await screen.findByRole('option', { name: 'option 20' })
  const viewport = option.closest<HTMLElement>('[data-radix-scroll-area-viewport]')
  expect(viewport).not.toBeNull()
  expect(viewport?.style.overflow).toBe('')
  fireEvent.click(option)
  rerender(<Select aria-label="scrolling options" value="20" onValueChange={() => {}} options={options} />)
  fireEvent.keyDown(screen.getByRole('combobox'), { key: 'ArrowDown' })
  await screen.findByRole('option', { name: 'option 20' })
  expect(errors).not.toHaveBeenCalled()
})
