// @vitest-environment jsdom
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { desktop, healthSchema, settingsSchema, type SettingsSession, type Snapshot } from './api'
import { applyLanguage } from './i18n'
import { SettingsWindow } from './SettingsWindow'

const backend = vi.hoisted(() => ({ listeners: new Map<string, (event: { data: unknown }) => void>() }))
vi.mock('@wailsio/runtime', () => ({
  Window: { IsMaximised: vi.fn().mockResolvedValue(false) },
  Call: { ByName: vi.fn() }, Clipboard: { SetText: vi.fn() }, Browser: { OpenURL: vi.fn() },
  Events: {
    On: (name: string, handle: (event: { data: unknown }) => void) => {
      backend.listeners.set(name, handle)
      return () => backend.listeners.delete(name)
    },
  },
}))

const settings = settingsSchema.parse({ language: 'en', proxies: [], activeProxyId: '', monitorId: '', tigerPort: 7819, tigerLinkGroup: 'A' })
const snapshot: Snapshot = {
  settings, rules: [], alerts: [],
  health: healthSchema.parse({ viaProxy: false, state: 'connected', detail: '', connections: 1, symbols: 0, ready: 0, updatedAtMs: 0, pingMs: null }),
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((complete) => { resolve = complete })
  return { promise, resolve }
}

async function emit(name: string, data: unknown) {
  await act(async () => { backend.listeners.get(name)?.({ data }) })
}

beforeEach(() => {
  applyLanguage('en')
  backend.listeners.clear()
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} })
  Element.prototype.scrollIntoView = vi.fn()
  vi.spyOn(desktop, 'snapshot').mockResolvedValue(snapshot)
  vi.spyOn(desktop, 'soundIds').mockResolvedValue(['chime'])
  vi.spyOn(desktop, 'monitors').mockResolvedValue([])
  vi.spyOn(desktop, 'hideSettings').mockResolvedValue(undefined)
  vi.spyOn(desktop, 'saveSettings').mockResolvedValue(undefined)
  vi.spyOn(desktop, 'settingsSession').mockResolvedValue({ section: 'general', revision: 1 })
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

it('loads standalone settings and hides the settings window after saving', async () => {
  const alertsAfter = vi.spyOn(desktop, 'alertsAfter')
  const sessionAlerts = vi.spyOn(desktop, 'sessionAlerts')
  render(<SettingsWindow />)
  const port = await screen.findByRole('spinbutton', { name: 'Tiger port' })
  expect(screen.queryByRole('dialog')).toBeNull()
  expect(screen.getByRole('tab', { name: 'general' }).getAttribute('data-state')).toBe('active')
  fireEvent.change(port, { target: { value: '8123' } })
  fireEvent.click(screen.getByRole('button', { name: 'save settings' }))
  await waitFor(() => expect(desktop.saveSettings).toHaveBeenCalledWith({ ...settings, tigerPort: 8123 }))
  await waitFor(() => expect(desktop.hideSettings).toHaveBeenCalledOnce())
  expect(alertsAfter).not.toHaveBeenCalled()
  expect(sessionAlerts).not.toHaveBeenCalled()
})

it('keeps a newer opening event when the initial session read finishes later', async () => {
  const initialSession = deferred<SettingsSession>()
  vi.mocked(desktop.settingsSession).mockReturnValue(initialSession.promise)
  render(<SettingsWindow />)
  await emit('settings:opened', { section: 'columns', revision: 2 })
  await screen.findByRole('region', { name: 'columns' })
  await act(async () => { initialSession.resolve({ section: 'general', revision: 1 }) })
  expect(screen.getByRole('tab', { name: 'columns' }).getAttribute('data-state')).toBe('active')
  expect(screen.queryByRole('spinbutton', { name: 'Tiger port' })).toBeNull()
})

it('resets unsaved forms and closes nested editors on every new opening', async () => {
  vi.mocked(desktop.settingsSession).mockResolvedValue({ section: 'alerts', revision: 1 })
  render(<SettingsWindow />)
  fireEvent.click(await screen.findByRole('button', { name: 'new alert' }))
  expect(screen.getByRole('dialog')).toBeTruthy()
  vi.mocked(desktop.snapshot).mockResolvedValue({ ...snapshot, settings: { ...settings, tigerPort: 9000 } })
  await emit('settings:opened', { section: 'general', revision: 2 })
  const port = await screen.findByRole('spinbutton', { name: 'Tiger port' })
  expect(screen.queryByRole('dialog')).toBeNull()
  expect(port.getAttribute('value')).toBe('9000')
  fireEvent.change(port, { target: { value: '9999' } })
  fireEvent.click(screen.getByRole('button', { name: 'close' }))
  await waitFor(() => expect(desktop.hideSettings).toHaveBeenCalledOnce())
  await emit('settings:opened', { section: 'general', revision: 3 })
  await waitFor(() => expect(screen.getByRole('spinbutton', { name: 'Tiger port' }).getAttribute('value')).toBe('9000'))
})

it('ignores an older pending snapshot after a new opening and preserves settings events', async () => {
  const stale = deferred<Snapshot>()
  vi.mocked(desktop.snapshot).mockReturnValueOnce(stale.promise).mockResolvedValue(snapshot)
  render(<SettingsWindow />)
  await waitFor(() => expect(desktop.snapshot).toHaveBeenCalledOnce())
  await emit('settings:opened', { section: 'columns', revision: 2 })
  await screen.findByRole('region', { name: 'columns' })
  await act(async () => { stale.resolve({ ...snapshot, settings: { ...settings, language: 'ru' } }) })
  expect(screen.getByRole('tab', { name: 'columns' }).getAttribute('data-state')).toBe('active')
  await emit('settings:changed', { ...settings, tableColumns: ['ticker'] })
  expect(screen.getByRole('group', { name: 'active columns' }).querySelectorAll('button')).toHaveLength(1)
})

it('keeps column changes made in another tab when saving general settings', async () => {
  render(<SettingsWindow />)
  await screen.findByRole('spinbutton', { name: 'Tiger port' })
  await emit('settings:changed', { ...settings, tableColumns: ['ticker'], soundMuted: true })
  fireEvent.click(screen.getByRole('button', { name: 'save settings' }))
  await waitFor(() => expect(desktop.saveSettings).toHaveBeenCalledWith(expect.objectContaining({ tableColumns: ['ticker'], soundMuted: true })))
})

it('keeps live settings delivered while an opening snapshot is pending', async () => {
  const pending = deferred<Snapshot>()
  vi.mocked(desktop.snapshot).mockReturnValue(pending.promise)
  vi.mocked(desktop.settingsSession).mockResolvedValue({ section: 'columns', revision: 1 })
  render(<SettingsWindow />)
  await waitFor(() => expect(desktop.snapshot).toHaveBeenCalledOnce())
  await emit('settings:changed', { ...settings, tableColumns: ['ticker'] })
  await act(async () => { pending.resolve(snapshot) })
  expect(await screen.findByRole('region', { name: 'columns' })).toBeTruthy()
  expect(screen.getByRole('group', { name: 'active columns' }).querySelectorAll('button')).toHaveLength(1)
})

it('keeps window controls available while settings are loading', async () => {
  vi.spyOn(desktop, 'snapshot').mockReturnValue(new Promise<Snapshot>(() => {}))
  const hide = vi.spyOn(desktop, 'hideSettings').mockResolvedValue(undefined)
  render(<SettingsWindow />)
  expect(screen.getByRole('tablist').closest('[data-window-toolbar]')).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: 'close' }))
  await waitFor(() => expect(hide).toHaveBeenCalledOnce())
})

it('places all settings tabs and the single close button in the title bar', async () => {
  render(<SettingsWindow />)
  await screen.findByRole('spinbutton', { name: 'Tiger port' })
  const toolbar = screen.getByRole('tablist').closest('[data-window-toolbar]')
  expect(toolbar?.querySelectorAll('[role="tab"]')).toHaveLength(5)
  expect(screen.getByRole('button', { name: 'close' }).closest('[data-window-toolbar]')).toBe(toolbar)
  expect(screen.getAllByRole('button', { name: 'close' })).toHaveLength(1)
  fireEvent.mouseDown(screen.getByRole('tab', { name: 'columns' }), { button: 0, ctrlKey: false })
  expect(await screen.findByRole('region', { name: 'columns' })).toBeTruthy()
  fireEvent.mouseDown(screen.getByRole('tab', { name: 'general' }), { button: 0, ctrlKey: false })
  expect(await screen.findByRole('spinbutton', { name: 'Tiger port' })).toBeTruthy()
})
