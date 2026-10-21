import { afterEach, expect, it, vi } from 'vitest'
import i18n, { applyLanguage, soundLabel } from './index'
import { en } from './en'
import { ru } from './ru'
import { localizeDiagnostic } from './diagnostics'
import { syncLanguage } from './language'
import type { Settings } from '../api'

const listeners = vi.hoisted(() => new Map<string, (event: { data: unknown }) => void>())
vi.mock('@wailsio/runtime', () => ({
  Call: {},
  Clipboard: {},
  Events: {
    On: (name: string, callback: (event: { data: unknown }) => void) => {
      listeners.set(name, callback)
      return () => listeners.delete(name)
    },
  },
}))
const settings = (language: 'en' | 'ru'): Settings => ({
  tableColumns: ['time', 'ticker', 'event', 'terminals'],
  language,
  proxies: [], activeProxyId: '',
  monitorId: '',
  tigerPort: 7819,
  tigerLinkGroup: 'A',
  soundMuted: false,
  bannersEnabled: true,
})
afterEach(() => {
  applyLanguage('en')
  listeners.clear()
})

it('provides a nonempty Russian translation for every English key', () => {
  expect(Object.keys(ru).sort()).toEqual(Object.keys(en).sort())
  expect(Object.values(ru).every((text) => text.trim().length > 0)).toBe(true)
})

it('preserves interpolation variables in both dictionaries', () => {
  for (const key of Object.keys(en)) {
    const english: Record<string, string> = en
    const russian: Record<string, string> = ru
    expect(russian[key].match(/{{\w+}}/g) ?? []).toEqual(english[key].match(/{{\w+}}/g) ?? [])
  }
})

it('translates controls while keeping sound names and user filenames intact', () => {
  applyLanguage('ru')
  expect(i18n.t('settings')).toBe('настройки')
  expect(i18n.t('openSymbol', { symbol: 'BTCUSDT' })).toBe('открыть BTCUSDT в терминале')
  for (const language of ['en', 'ru'] as const) {
    applyLanguage(language)
    for (const sound of ['bell', 'chime', 'notification', 'alarm', 'beep']) {
      expect(soundLabel(sound)).toBe(sound)
    }
    expect(soundLabel('custom:123-my-sound.wav')).toBe('my-sound.wav')
  }
})

it('translates diagnostics without changing terminal URLs or API responses', () => {
  applyLanguage('ru')
  expect(localizeDiagnostic('Error: invalid alert rule')).toBe('проверьте настройки алерта')
  expect(localizeDiagnostic('book stream: failed to read: use of closed network connection')).toBe(
    'поток цен: соединение закрыто',
  )
  const endpoint = 'http://localhost:5005/api/open/zenica/binance/F/BTCUSDT'
  expect(localizeDiagnostic(`Post "${endpoint}": connection refused`)).toContain(endpoint)
  expect(localizeDiagnostic('Tiger 400: Symbol not found')).toBe('Tiger 400: Symbol not found')
})

it('does not let a late startup snapshot overwrite a newer language event', async () => {
  let resolveInitial: ((value: Settings) => void) | undefined
  const initial = new Promise<Settings>((resolve) => {
    resolveInitial = resolve
  })
  const stop = syncLanguage(initial, (error) => {
    throw error
  })
  listeners.get('settings:changed')?.({ data: settings('ru') })
  resolveInitial?.(settings('en'))
  await initial
  await Promise.resolve()
  expect(i18n.language).toBe('ru')
  stop()
  expect(listeners.size).toBe(0)
})
