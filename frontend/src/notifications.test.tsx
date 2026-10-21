// @vitest-environment jsdom
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { applyLanguage } from './i18n'
import type { Alert, Rule, Snapshot } from './api'
import { Overlay } from './Overlay'
import { RuleDialog } from './components/RuleDialog'
import { RulesSettings } from './components/RulesSettings'
import { ConnectionStatus } from './components/ConnectionStatus'
import { ProxySettings } from './components/ProxySettings'
import { listingExchanges } from './listings'

const backend = vi.hoisted(() => ({
  call: vi.fn(),
  openURL: vi.fn().mockResolvedValue(undefined),
  listeners: new Map<string, (event: { data: unknown }) => void>(),
}))
vi.mock('@wailsio/runtime', () => ({
  Call: { ByName: backend.call },
  Clipboard: { SetText: vi.fn() },
  Browser: { OpenURL: backend.openURL },
  Events: {
    On: (name: string, callback: (event: { data: unknown }) => void) => {
      backend.listeners.set(name, callback)
      return () => backend.listeners.delete(name)
    },
  },
}))

const rule: Rule = {
  bannerSeconds: -1,
  openExchanges: [],
  id: 1,
  name: 'knife',
  percent: 3,
  windowSeconds: 60,
  color: '#41c977',
  volume: 0.5,
  minQuoteVolume: 80e6,
  terminals: ['vataga'],
  sound: 'chime',
}
const alert: Alert = {
  bannerSeconds: -1,
  openExchanges: [],
  receivedAtMs: 1, detectedAtMs: 1, writeStartedAtMs: 1,
  id: 1,
  ruleId: 1,
  ruleName: 'knife',
  symbol: 'BTCUSDT',
  direction: 'down',
  changePercent: -3,
  price: 90,
  spreadPercent: 0.01,
  quoteVolume: 100e6,
  eventTimeMs: 1,
  createdAtMs: 1,
  sound: 'chime',
  volume: 0.5,
  color: '#41c977',
  terminals: ['vataga'],
  terminalStatus: 'ok',
  terminalError: '',
}

const initialSnapshot = (): Snapshot => ({
  rules: [rule],
  alerts: [alert],
  settings: {
    language: 'en',
    proxies: [],
    activeProxyId: '',
    monitorId: '',
    tigerPort: 7819,
    tigerLinkGroup: 'A',
    soundMuted: false,
    bannersEnabled: true,
  },
  health: {
    timings: [], quoteAgeMs: 50, streams: [], storage: { size: 0, oldestMs: 0, writeMs: 0, unsaved: 0, error: '' },
    volumeAgeMs: 0, serviceError: '', usedWeight: 0, terminalQueued: 0, terminalErrors: 0,
    renderDelayMs: 0, terminalWaitMs: 0, terminalRequestMs: 0,
    state: 'connected',
    viaProxy: false,
    detail: 'signals live',
    connections: 4,
    symbols: 749,
    ready: 10,
    pingMs: 50,
    updatedAtMs: 1,
  },
})

beforeEach(() => {
  applyLanguage('en')
  backend.call.mockReset()
  backend.openURL.mockClear()
  backend.listeners.clear()
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  )
  window.matchMedia = vi.fn().mockReturnValue({
    matches: true,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  })
  backend.call.mockImplementation(async (name: string, ...args: unknown[]) => {
    if (name.endsWith('.GetSnapshot')) return initialSnapshot()
    if (name.endsWith('.SessionAlerts')) return [alert]
    if (name.endsWith('.SaveRule')) return args[0]
    if (name.endsWith('.OpenTerminals')) return { opened: 1, error: '' }
  })
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

it('dismisses a notification after a successful manual open', async () => {
  render(<Overlay />)
  await userEvent.click(await screen.findByRole('button', { name: 'open BTCUSDT in terminal' }))
  await waitFor(() => expect(screen.queryByRole('button', { name: 'open BTCUSDT in terminal' })).toBeNull())
  expect(backend.call).toHaveBeenCalledWith('main.DesktopService.OpenTerminals', 1, 'BTCUSDT', ['vataga'])
})

it('keeps a failed notification available for retry', async () => {
  backend.call.mockImplementation(async (name: string) => {
    if (name.endsWith('.GetSnapshot')) return initialSnapshot()
    if (name.endsWith('.SessionAlerts')) return [alert]
    if (name.endsWith('.OpenTerminals')) return { opened: 0, error: 'Vataga unavailable' }
  })
  render(<Overlay />)
  await userEvent.click(await screen.findByRole('button', { name: 'open BTCUSDT in terminal' }))
  expect(await screen.findByText(/Vataga unavailable/)).toBeTruthy()
  expect(screen.getByRole('button', { name: 'open BTCUSDT in terminal' })).toBeTruthy()
})

it('saves color, volume, turnover and terminal choices without an enable switch', async () => {
  render(<RuleDialog open onOpenChange={vi.fn()} rule={rule} sounds={['chime']} onSaved={vi.fn()} />)
  expect(screen.queryByRole('checkbox')).toBeNull()
  await userEvent.click(screen.getByRole('button', { name: 'alert color' }))
  await userEvent.click(screen.getByRole('button', { name: 'cyan' }))
  await userEvent.click(screen.getByRole('button', { name: 'auto-open in Tiger' }))
  await userEvent.clear(screen.getByLabelText('min 24h turnover, million USD'))
  await userEvent.type(screen.getByLabelText('min 24h turnover, million USD'), '120')
  await userEvent.click(screen.getByRole('button', { name: 'preview sound' }))
  expect(backend.call).toHaveBeenCalledWith('main.DesktopService.PreviewSound', 'chime', 0.5)
  await userEvent.click(screen.getByRole('button', { name: 'save alert' }))
  await waitFor(() =>
    expect(backend.call).toHaveBeenCalledWith('main.DesktopService.SaveRule', {
      ...rule,
      color: '#46c6df',
      terminals: ['vataga', 'tiger'],
      minQuoteVolume: 120e6,
    }),
  )
  expect(document.querySelector('input[type=file]')?.getAttribute('accept')).toContain('.mp3,.wav')
})

it('explains the automatic cooldown beside the rule window', async () => {
  render(<RuleDialog open rule={rule} sounds={['chime']} onSaved={vi.fn()} onOpenChange={vi.fn()} />)
  const trigger = screen.getByRole('button', { name: 'about the window and repeat cooldown' })
  expect(trigger.getAttribute('title')).toBeNull()
  await userEvent.hover(trigger)
  expect((await screen.findByRole('tooltip')).textContent).toContain(
    'After a signal, the same ticker and rule pause for that many seconds, in both directions.',
  )
})

it('saves the language preference in settings', async () => {
  const { SettingsDialog } = await import('./components/SettingsDialog')
  const settings = {
    language: 'en' as const,
    proxies: [],
    activeProxyId: '',
    monitorId: '',
    tigerPort: 7819,
    tigerLinkGroup: 'A',
    soundMuted: false,
    bannersEnabled: true,
  }
  backend.call.mockImplementation(async (name: string) => {
    if (name.endsWith('.Monitors')) return []
  })
  render(
    <SettingsDialog
      open
      onOpenChange={vi.fn()}
      settings={settings}
      rules={[]}
      sounds={['chime']}
      onSaved={vi.fn()}
    />,
  )
  await userEvent.click(screen.getByRole('tab', { name: 'general' }))
  expect(screen.queryByRole('spinbutton', { name: /cooldown/i })).toBeNull()
  await userEvent.selectOptions(screen.getByLabelText('language'), 'ru')
  expect(screen.getByRole('button', { name: 'save settings' })).toBeTruthy()
  await userEvent.click(screen.getByRole('button', { name: 'save settings' }))
  await waitFor(() =>
    expect(backend.call).toHaveBeenCalledWith('main.DesktopService.SaveSettings', {
      ...settings,
      language: 'ru',
    }),
  )
})

it('switches the overlay language live without losing its cards', async () => {
  const { act } = await import('@testing-library/react')
  render(<Overlay />)
  await screen.findByRole('button', { name: 'open BTCUSDT in terminal' })
  await act(async () => {
    backend.listeners.get('settings:changed')?.({
      data: { ...initialSnapshot().settings, language: 'ru' },
    })
  })
  expect(screen.getByRole('button', { name: 'открыть BTCUSDT в терминале' })).toBeTruthy()
  expect(screen.getByText('knife')).toBeTruthy()
  expect(document.documentElement.lang).toBe('ru')
  await act(async () => {
    backend.listeners.get('settings:changed')?.({
      data: initialSnapshot().settings,
    })
  })
  expect(screen.getByRole('button', { name: 'open BTCUSDT in terminal' })).toBeTruthy()
})

it('renders the Russian editor without changing stored alert values', async () => {
  applyLanguage('ru')
  render(<RuleDialog open onOpenChange={vi.fn()} rule={rule} sounds={['chime']} onSaved={vi.fn()} />)
  expect(screen.getByRole('heading', { name: 'настройка алерта' })).toBeTruthy()
  expect(screen.getByLabelText('название').getAttribute('value')).toBe('knife')
  expect(screen.getByRole('option', { name: 'chime' }).getAttribute('value')).toBe('chime')
  expect(screen.getByRole('button', { name: 'автооткрытие в Vataga' })).toBeTruthy()
  await userEvent.click(screen.getByRole('button', { name: 'сохранить' }))
  await waitFor(() => expect(backend.call).toHaveBeenCalledWith('main.DesktopService.SaveRule', rule))
})

it('places the new alert button below all rule cards', () => {
  const { container } = render(
    <RulesSettings
      rules={[rule, { ...rule, id: 2, name: 'second' }]}
      sounds={['chime']}
      onSaved={() => {}}
    />,
  )
  const list = container.querySelector('.rule-list')
  expect(list?.lastElementChild).toBe(screen.getByRole('button', { name: 'new alert' }))
  expect(list?.querySelectorAll('.rule-card')).toHaveLength(2)
})

it('saves silent notifications without exposing volume or playback controls', async () => {
  const user = userEvent.setup()
  render(<RuleDialog open onOpenChange={() => {}} rule={rule} sounds={['chime']} onSaved={() => {}} />)
  await user.selectOptions(screen.getByLabelText('sound'), 'none')
  expect(screen.queryByRole('slider', { name: 'sound volume' })).toBeNull()
  expect(screen.getByRole('button', { name: 'preview sound' }).hasAttribute('disabled')).toBe(true)
  await user.click(screen.getByRole('button', { name: 'save alert' }))
  await waitFor(() => {
    const saved = backend.call.mock.calls.find(([name]) => String(name).endsWith('.SaveRule'))
    expect(saved?.[1]).toMatchObject({ sound: 'none', volume: 0.5 })
  })
})

it('shows clear colored connection states without a warmup message', () => {
  applyLanguage('ru')
  const health = initialSnapshot().health
  const { rerender } = render(<ConnectionStatus health={{ ...health, state: 'error' }} />)
  expect(screen.getByRole('status').textContent).toBe('нет соединения с Binance')
  expect(screen.getByRole('status').className).toContain('latency-offline')
  rerender(<ConnectionStatus health={{ ...health, state: 'reconnecting' }} />)
  expect(screen.getByRole('status').textContent).toBe('подключение к Binance')
  expect(screen.getByRole('status').className).toContain('latency-warning')
  rerender(<ConnectionStatus health={{ ...health, state: 'warming' }} />)
  expect(screen.getByRole('status').textContent).toBe('50 мс')
  expect(screen.queryByText('прогрев')).toBeNull()
  expect(screen.getByRole('status').className).toContain('latency-good')
})

it('creates a new rule through the full settings form', async () => {
  const user = userEvent.setup()
  const saved = vi.fn()
  render(<RulesSettings rules={[]} sounds={['chime', 'alarm']} onSaved={saved} />)
  await user.click(screen.getByRole('button', { name: 'new alert' }))
  await user.clear(screen.getByLabelText('name'))
  await user.type(screen.getByLabelText('name'), 'fast knife')
  await user.clear(screen.getByLabelText('movement, %'))
  await user.type(screen.getByLabelText('movement, %'), '10')
  await user.clear(screen.getByLabelText('window, seconds'))
  await user.type(screen.getByLabelText('window, seconds'), '300')
  await user.selectOptions(screen.getByLabelText('sound'), 'alarm')
  await user.click(screen.getByRole('button', { name: 'auto-open in Vataga' }))
  await user.click(screen.getByRole('button', { name: 'save alert' }))
  await waitFor(() => expect(saved).toHaveBeenCalledOnce())
  expect(backend.call).toHaveBeenCalledWith('main.DesktopService.SaveRule', {
    id: 0,
    kind: 'price_move', listingExchanges: [], listingTypes: [],
    bannerSeconds: 15,
    openExchanges: [],
    name: 'fast knife',
    percent: 10,
    windowSeconds: 300,
    sound: 'alarm',
    color: '#fa423e',
    volume: 1,
    minQuoteVolume: 80e6,
    terminals: ['vataga'],
  })
  expect(screen.queryByRole('dialog')).toBeNull()
})

it('keeps the draft and error visible when saving fails, then allows retry', async () => {
  const user = userEvent.setup()
  const onSaved = vi.fn()
  const onOpenChange = vi.fn()
  backend.call.mockRejectedValueOnce(new Error('invalid alert rule'))
  render(<RuleDialog open onOpenChange={onOpenChange} rule={rule} sounds={['chime']} onSaved={onSaved} />)
  await user.click(screen.getByRole('button', { name: 'save alert' }))
  expect(await screen.findByText('invalid alert settings')).toBeTruthy()
  expect(onSaved).not.toHaveBeenCalled()
  expect(onOpenChange).not.toHaveBeenCalled()
  expect(screen.getByLabelText('name').getAttribute('value')).toBe('knife')
  await user.click(screen.getByRole('button', { name: 'save alert' }))
  await waitFor(() => expect(onSaved).toHaveBeenCalledOnce())
  expect(onOpenChange).toHaveBeenCalledWith(false)
})

it('requires confirmation before deleting a rule', async () => {
  const user = userEvent.setup()
  const saved = vi.fn()
  render(<RuleDialog open onOpenChange={vi.fn()} rule={rule} sounds={['chime']} onSaved={saved} />)
  await user.click(screen.getByRole('button', { name: 'delete' }))
  expect(backend.call).not.toHaveBeenCalled()
  await user.click(screen.getByRole('button', { name: 'confirm delete' }))
  await waitFor(() => expect(saved).toHaveBeenCalledOnce())
  expect(backend.call).toHaveBeenCalledWith('main.DesktopService.DeleteRule', rule.id)
})

it('renders new event notifications and retains only three during a burst', async () => {
  backend.call.mockImplementation(async (name: string) => {
    if (name.endsWith('.GetSnapshot')) return initialSnapshot()
    if (name.endsWith('.SessionAlerts')) return []
  })
  render(<Overlay />)
  await waitFor(() => expect(backend.listeners.has('alert:new')).toBe(true))
  await act(async () => {
    for (let id = 1; id <= 15; id++) {
      backend.listeners.get('alert:new')?.({ data: { ...alert, id, symbol: `COIN${id}USDT` } })
    }
  })
  await waitFor(() => expect(screen.getAllByRole('button', { name: /open COIN/ })).toHaveLength(3))
  for (const id of [13, 14, 15])
    expect(screen.getByRole('button', { name: `open COIN${id}USDT in terminal` })).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'open COIN1USDT in terminal' })).toBeNull()
})

it('creates a proxy with the use checkbox enabled by default and allows opting out', async () => {
  const user = userEvent.setup()
  const saved = vi.fn()
  render(<ProxySettings settings={initialSnapshot().settings} onSaved={saved} />)
  await user.click(screen.getByRole('button', { name: 'add proxy' }))
  expect(screen.getByLabelText('name').getAttribute('value')).toBe('proxy-1')
  const checkbox = screen.getByRole('checkbox', { name: 'use this proxy' })
  expect(screen.queryByRole('combobox')).toBeNull()
  expect(checkbox).toHaveProperty('checked', true)
  await user.type(screen.getByLabelText('ip or hostname'), 'route.example')
  await user.type(screen.getByLabelText('login'), 'test-user')
  await user.type(screen.getByLabelText('password'), 'test-password')
  expect(screen.getByLabelText('password').getAttribute('type')).toBe('password')
  await user.click(checkbox)
  await user.click(screen.getByRole('button', { name: 'save proxy' }))
  await waitFor(() => expect(saved).toHaveBeenCalledOnce())
  expect(backend.call).toHaveBeenCalledWith(
    'main.DesktopService.SaveProxy',
    {
      id: '',
      name: 'proxy-1',
      host: 'route.example',
      port: 443,
      protocol: 'http',
      login: 'test-user',
      password: 'test-password',
    },
    false,
  )
})

it('marks only the active proxy card green and restores direct mode on selection', async () => {
  const user = userEvent.setup()
  const settings = {
    ...initialSnapshot().settings,
    proxies: [
      {
        id: 'one',
        name: 'proxy-1',
        host: 'localhost',
        port: 1080,
        protocol: 'socks5' as const,
        login: '',
        password: '',
      },
      {
        id: 'two',
        name: 'proxy-2',
        host: 'route.example',
        port: 443,
        protocol: 'https' as const,
        login: '',
        password: '',
      },
    ],
    activeProxyId: 'two',
  }
  const { container } = render(<ProxySettings settings={settings} onSaved={vi.fn()} />)
  const active = container.querySelectorAll('.proxy-card.is-active')
  expect(active).toHaveLength(1)
  expect(active[0].textContent).toContain('proxy-2')
  await user.click(screen.getByRole('button', { name: 'no proxy' }))
  expect(backend.call).toHaveBeenCalledWith('main.DesktopService.SelectProxy', '')
})

it('shows inactive proxy checkbox as unchecked and enables it on save', async () => {
  const user = userEvent.setup()
  const settings = {
    ...initialSnapshot().settings,
    proxies: [
      {
        id: 'one',
        name: 'proxy-1',
        host: 'localhost',
        port: 1080,
        protocol: 'socks5' as const,
        login: '',
        password: '',
      },
    ],
    activeProxyId: '',
  }
  render(<ProxySettings settings={settings} onSaved={vi.fn()} />)
  await user.click(screen.getByRole('button', { name: /proxy-1/ }))
  const checkbox = screen.getByRole('checkbox', { name: 'use this proxy' })
  expect(checkbox).toHaveProperty('checked', false)
  await user.click(checkbox)
  await user.click(screen.getByRole('button', { name: 'save proxy' }))
  await waitFor(() =>
    expect(backend.call).toHaveBeenCalledWith('main.DesktopService.SaveProxy', settings.proxies[0], true),
  )
})

it('offers manual reconnect for both exchange and proxy failures', async () => {
  const user = userEvent.setup()
  const { rerender } = render(<ConnectionStatus health={{ ...initialSnapshot().health, state: 'error' }} />)
  await user.click(screen.getByRole('button', { name: 'reconnect' }))
  expect(backend.call).toHaveBeenCalledWith('main.DesktopService.Reconnect')
  rerender(<ConnectionStatus health={{ ...initialSnapshot().health, state: 'error', viaProxy: true }} />)
  expect(screen.getByText('proxy connection unavailable')).toBeTruthy()
  expect(screen.getByRole('button', { name: 'reconnect' })).toBeTruthy()
})

it('fills a proxy from pasted details and saves it enabled by default', async () => {
  const user = userEvent.setup()
  render(<ProxySettings settings={initialSnapshot().settings} onSaved={vi.fn()} />)
  await user.click(screen.getByRole('button', { name: 'add proxy' }))
  await user.click(screen.getByLabelText('ip or hostname'))
  await user.paste('route.example : 443\nЛогин: test-user\nПароль: test-password')
  expect(screen.getByLabelText('ip or hostname').getAttribute('value')).toBe('route.example')
  expect(screen.getByLabelText('login').getAttribute('value')).toBe('test-user')
  expect(screen.getByLabelText('password').getAttribute('value')).toBe('test-password')
  await user.click(screen.getByRole('button', { name: 'save proxy' }))
  await waitFor(() =>
    expect(backend.call).toHaveBeenCalledWith(
      'main.DesktopService.SaveProxy',
      {
        id: '',
        name: 'proxy-1',
        host: 'route.example',
        port: 443,
        protocol: 'http',
        login: 'test-user',
        password: 'test-password',
      },
      true,
    ),
  )
})

it('uses local SOCKS automatically when credentials are empty', async () => {
  const user = userEvent.setup()
  render(<ProxySettings settings={initialSnapshot().settings} onSaved={vi.fn()} />)
  await user.click(screen.getByRole('button', { name: 'add proxy' }))
  await user.click(screen.getByLabelText('ip or hostname'))
  await user.paste('127.0.0.1:1080')
  await user.click(screen.getByRole('button', { name: 'save proxy' }))
  await waitFor(() =>
    expect(backend.call).toHaveBeenCalledWith(
      'main.DesktopService.SaveProxy',
      {
        id: '',
        name: 'proxy-1',
        host: '127.0.0.1',
        port: 1080,
        protocol: 'socks5',
        login: '',
        password: '',
      },
      true,
    ),
  )
})


it('shows median transport ping and keeps quote age in the tooltip', () => {
  const health = { ...initialSnapshot().health, quoteAgeMs: 5850, pingMs: 20 }
  const { rerender } = render(<ConnectionStatus health={health} />)
  expect(screen.getByRole('status').textContent).toBe('20 ms')
  expect(screen.getByRole('status').className).toContain('latency-good')
  expect(screen.getByRole('status').title).toContain('5850 ms')
  rerender(<ConnectionStatus health={{ ...health, quoteAgeMs: 80 }} />)
  expect(screen.getByRole('status').className).toContain('latency-good')
})


it('acknowledges the overlay browser frame separately from feed rendering', async () => {
  render(<Overlay />)
  await waitFor(() => expect(backend.call).toHaveBeenCalledWith('main.DesktopService.AlertRendered', alert.id, 'overlay'))
})

it('creates an external listing rule without price or turnover conditions', async () => {
  render(<RuleDialog open onOpenChange={vi.fn()} rule={null} sounds={['chime']} onSaved={vi.fn()} />)
  await userEvent.selectOptions(screen.getByLabelText('alert type'), 'listing')
  await userEvent.clear(screen.getByLabelText('name'))
  await userEvent.type(screen.getByLabelText('name'), 'external listings')
  expect(screen.queryByLabelText('movement, %')).toBeNull()
  expect(screen.queryByLabelText('min 24h turnover, million USD')).toBeNull()
  expect(screen.queryByRole('checkbox', { name: 'all external exchanges' })).toBeNull()
  const cards = screen.getByRole('group', { name: 'listing exchanges' }).querySelectorAll('[role="checkbox"]')
  expect(cards).toHaveLength(listingExchanges.length)
  for (const card of cards) {
    expect(card.getAttribute('aria-checked')).toBe('true')
    expect(card.className).toContain('exchange-card is-active')
    expect(card.querySelector('img')).toBeTruthy()
  }
  await userEvent.click(screen.getByRole('button', { name: 'save alert' }))
  await waitFor(() => expect(backend.call).toHaveBeenCalledWith('main.DesktopService.SaveRule', expect.objectContaining({
    kind: 'listing', listingExchanges: [], listingTypes: ['spot'], name: 'external listings',
  })))
})

it('uses the alert type as the default name without overwriting custom names', async () => {
  applyLanguage('ru')
  render(<RuleDialog open onOpenChange={vi.fn()} rule={null} sounds={['chime']} onSaved={vi.fn()} />)
  const name = screen.getByLabelText('название')
  const type = screen.getByLabelText('тип алерта')
  expect(name).toHaveProperty('value', 'движение цены')
  await userEvent.selectOptions(type, 'listing')
  expect(name).toHaveProperty('value', 'листинг')
  expect(screen.getByRole('group', { name: 'типы событий' })).toBeTruthy()
  for (const event of ['Листинг на споте', 'Листинг на фьючерсах', 'Анонс листинга', 'Открытие премаркета', 'Скорый листинг на споте', 'Добавление в каталог']) {
    expect(screen.getByRole('checkbox', { name: event })).toBeTruthy()
  }
  await userEvent.click(screen.getByRole('button', { name: 'сохранить' }))
  await waitFor(() => expect(backend.call).toHaveBeenCalledWith('main.DesktopService.SaveRule', expect.objectContaining({ kind: 'listing', name: 'листинг' })))
  await userEvent.clear(name)
  await userEvent.type(name, 'только Upbit')
  await userEvent.selectOptions(type, 'price_move')
  expect(name).toHaveProperty('value', 'только Upbit')
})

it('lets a listing rule exclude an exchange and restore all exchanges', async () => {
  render(<RuleDialog open onOpenChange={vi.fn()} rule={null} sounds={['chime']} onSaved={vi.fn()} />)
  await userEvent.selectOptions(screen.getByLabelText('alert type'), 'listing')
  await userEvent.clear(screen.getByLabelText('name'))
  await userEvent.type(screen.getByLabelText('name'), 'external listings')
  await userEvent.click(screen.getByRole('checkbox', { name: 'Upbit' }))
  expect(screen.getByRole('checkbox', { name: 'Upbit' }).getAttribute('aria-checked')).toBe('false')
  await userEvent.click(screen.getByRole('button', { name: 'save alert' }))
  await waitFor(() => expect(backend.call).toHaveBeenCalledWith('main.DesktopService.SaveRule', expect.objectContaining({
    listingExchanges: listingExchanges.filter((exchange) => exchange !== 'upbit'),
  })))
  await userEvent.click(screen.getByRole('checkbox', { name: 'Upbit' }))
  await userEvent.click(screen.getByRole('button', { name: 'save alert' }))
  await waitFor(() => expect(backend.call).toHaveBeenLastCalledWith('main.DesktopService.SaveRule', expect.objectContaining({ listingExchanges: [] })))
})

it('toggles exchange cards and keeps at least one market type selected', async () => {
  render(<RuleDialog open onOpenChange={vi.fn()} rule={{ ...rule, kind: 'listing', listingExchanges: ['upbit'], listingTypes: ['spot'] }} sounds={['chime']} onSaved={vi.fn()} />)
  expect(screen.getByRole('checkbox', { name: 'Upbit' }).getAttribute('aria-checked')).toBe('true')
  expect(screen.getByRole('checkbox', { name: 'Spot listing' })).toHaveProperty('disabled', true)
  await userEvent.click(screen.getByRole('checkbox', { name: 'Bithumb' }))
  expect(screen.getByRole('checkbox', { name: 'Upbit' })).toHaveProperty('disabled', false)
  await userEvent.click(screen.getByRole('checkbox', { name: 'Upbit' }))
  expect(screen.getByRole('checkbox', { name: 'Bithumb' }).getAttribute('aria-checked')).toBe('true')
})

it('renders external listings with the source exchange instead of a fake percentage', async () => {
  const listing: Alert = { ...alert, kind: 'listing', direction: '', changePercent: 0, symbol: 'REZUSDT', listing: {
    exchange: 'upbit', asset: 'REZ', marketType: 'spot', markets: ['krw'], url: 'https://upbit.com/notice?id=1', detectedTimeUs: 1, sentTimeUs: 3000001,
  } }
  backend.call.mockImplementation(async (name: string) => {
    if (name.endsWith('.GetSnapshot')) return initialSnapshot()
    if (name.endsWith('.SessionAlerts')) return [listing]
  })
  render(<Overlay />)
  expect(await screen.findByText('Upbit')).toBeTruthy()
  expect(screen.getByText(/Spot listing/)).toBeTruthy()
  expect(screen.queryByText('0.00%')).toBeNull()
  expect(screen.queryByRole('button', { name: 'open listing announcement' })).toBeNull()
})

it('configures the listing source without reading a saved key into the form', async () => {
  const { ListingSourceSettings } = await import('./components/ListingSourceSettings')
  const source = { endpoint: 'wss://ws.newlistings.pro/v2/full', keyConfigured: true, health: {
    state: 'connected', code: '', plan: 'feed_free', delayMs: 3000, received: 0, rejected: 0, unmatched: 0, lastEventAtMs: 0, providerWaitMs: 0, deliveryMs: 0,
  } }
  backend.call.mockImplementation(async (name: string) => {
    if (name.endsWith('.ListingSource')) return source
  })
  render(<ListingSourceSettings />)
  await screen.findByPlaceholderText('key saved')
  expect(screen.queryByRole('combobox')).toBeNull()
  expect(screen.queryByText(/feed_free/)).toBeNull()
  const bot = screen.getByRole('link', { name: '@NLF_websocket_bot' })
  expect(bot.getAttribute('href')).toBe('https://t.me/NLF_websocket_bot')
  await userEvent.click(bot)
  expect(backend.openURL).toHaveBeenCalledWith('https://t.me/NLF_websocket_bot')
  const input = screen.getByLabelText('API key')
  expect(input).toHaveProperty('value', '')
  expect(input.getAttribute('type')).toBe('password')
  await userEvent.type(input, 'unit-test-new-key')
  await userEvent.click(screen.getByRole('button', { name: 'save' }))
  await waitFor(() => expect(backend.call).toHaveBeenCalledWith('main.DesktopService.SaveListingSource', source.endpoint, 'unit-test-new-key'))
  await waitFor(() => expect(input).toHaveProperty('value', ''))
})


it('toggles global sound with green and red states and reports save failures', async () => {
  const { SoundToggle } = await import('./components/SoundToggle')
  const onError = vi.fn()
  const settings = initialSnapshot().settings
  const { rerender } = render(<SoundToggle settings={settings} onError={onError} />)
  expect(screen.getByRole('button', { name: 'mute sounds' }).className).toContain('sound-enabled')
  await userEvent.click(screen.getByRole('button', { name: 'mute sounds' }))
  await waitFor(() => expect(backend.call).toHaveBeenCalledWith('main.DesktopService.SetSoundMuted', true))
  rerender(<SoundToggle settings={{ ...settings, soundMuted: true }} onError={onError} />)
  expect(screen.getByRole('button', { name: 'unmute sounds' }).className).toContain('sound-muted')
  expect(screen.getByRole('button').getAttribute('aria-pressed')).toBe('true')
  backend.call.mockRejectedValueOnce(new Error('save failed'))
  await userEvent.click(screen.getByRole('button', { name: 'unmute sounds' }))
  await waitFor(() => expect(onError).toHaveBeenCalledWith('Error: save failed'))
  expect(screen.getByRole('button').className).toContain('sound-muted')
})

it('saves banner visibility independently of sound in general settings', async () => {
  const { SettingsDialog } = await import('./components/SettingsDialog')
  const settings = { ...initialSnapshot().settings, soundMuted: true }
  backend.call.mockResolvedValue([])
  render(<SettingsDialog open onOpenChange={vi.fn()} settings={settings} rules={[]} sounds={[]} onSaved={vi.fn()} />)
  await userEvent.click(screen.getByRole('tab', { name: 'general' }))
  const checkbox = screen.getByRole('checkbox', { name: 'corner notification banners' })
  expect(checkbox.matches(':checked')).toBe(true)
  await userEvent.click(checkbox)
  await userEvent.click(screen.getByRole('button', { name: 'save settings' }))
  await waitFor(() => expect(backend.call).toHaveBeenCalledWith('main.DesktopService.SaveSettings', { ...settings, bannersEnabled: false }))
})

it('clears banners when disabled and only shows new alerts when enabled again', async () => {
  render(<Overlay />)
  await screen.findByRole('button', { name: 'open BTCUSDT in terminal' })
  await act(async () => {
    backend.listeners.get('settings:changed')?.({ data: { ...initialSnapshot().settings, bannersEnabled: false } })
    backend.listeners.get('alert:new')?.({ data: { ...alert, id: 2 } })
  })
  await waitFor(() => expect(screen.queryByRole('button', { name: 'open BTCUSDT in terminal' })).toBeNull())
  await act(async () => {
    backend.listeners.get('settings:changed')?.({ data: initialSnapshot().settings })
  })
  expect(screen.queryByRole('button', { name: 'open BTCUSDT in terminal' })).toBeNull()
  await act(async () => {
    backend.listeners.get('alert:new')?.({ data: { ...alert, id: 3 } })
  })
  await screen.findByRole('button', { name: 'open BTCUSDT in terminal' })
})

it('does not restore session banners when banners are disabled at startup', async () => {
  backend.call.mockImplementation(async (name: string) => {
    if (name.endsWith('.GetSnapshot')) return { ...initialSnapshot(), settings: { ...initialSnapshot().settings, bannersEnabled: false } }
    if (name.endsWith('.SessionAlerts')) return [alert]
  })
  render(<Overlay />)
  await act(async () => {})
  expect(screen.queryByRole('button', { name: 'open BTCUSDT in terminal' })).toBeNull()
  expect(backend.call).not.toHaveBeenCalledWith('main.DesktopService.SessionAlerts')
})


it('discards pending session banners after notifications are toggled off and on', async () => {
  let resolveSession: (alerts: Alert[]) => void = () => { throw new Error('session request not started') }
  backend.call.mockImplementation(async (name: string) => {
    if (name.endsWith('.GetSnapshot')) return initialSnapshot()
    if (name.endsWith('.SessionAlerts')) return new Promise<Alert[]>((resolve) => { resolveSession = resolve })
  })
  render(<Overlay />)
  await waitFor(() => expect(backend.call).toHaveBeenCalledWith('main.DesktopService.SessionAlerts'))
  await act(async () => {
    backend.listeners.get('settings:changed')?.({ data: { ...initialSnapshot().settings, bannersEnabled: false } })
    backend.listeners.get('settings:changed')?.({ data: initialSnapshot().settings })
    resolveSession([alert])
  })
  expect(screen.queryByRole('button', { name: 'open BTCUSDT in terminal' })).toBeNull()
})

it.each([
  [0, 0, 'do not show'], [1, 5, '5 seconds'], [2, 15, '15 seconds'],
  [3, 30, '30 seconds'], [4, 60, '1 minute'], [5, 300, '5 minutes'],
  [6, -1, 'keep until closed'],
])('saves banner slider step %s as %s seconds', async (index, seconds, label) => {
  render(<RuleDialog open onOpenChange={vi.fn()} rule={rule} sounds={['chime']} onSaved={vi.fn()} />)
  const slider = screen.getByRole('slider', { name: 'banner display time' })
  expect(slider.getAttribute('min')).toBe('0')
  expect(slider.getAttribute('max')).toBe('6')
  expect(slider.getAttribute('step')).toBe('1')
  fireEvent.change(slider, { target: { value: String(index) } })
  expect(slider.getAttribute('aria-valuetext')).toBe(label)
  await userEvent.click(screen.getByRole('button', { name: 'save alert' }))
  await waitFor(() => expect(backend.call).toHaveBeenCalledWith('main.DesktopService.SaveRule', { ...rule, bannerSeconds: seconds }))
})

it('does not render feed-only alerts as banners', async () => {
  backend.call.mockImplementation(async (name: string) => {
    if (name.endsWith('.GetSnapshot')) return initialSnapshot()
    if (name.endsWith('.SessionAlerts')) return [{ ...alert, bannerSeconds: 0 }]
  })
  render(<Overlay />)
  await act(async () => {})
  expect(screen.queryByRole('button', { name: 'open BTCUSDT in terminal' })).toBeNull()
  await act(async () => {
    backend.listeners.get('alert:new')?.({ data: { ...alert, id: 2, bannerSeconds: 0 } })
    await new Promise((resolve) => setTimeout(resolve, 60))
  })
  expect(screen.queryByRole('button', { name: 'open BTCUSDT in terminal' })).toBeNull()
})

it('closes timed banners without resetting their timers when other alerts arrive', async () => {
  vi.useFakeTimers()
  backend.call.mockImplementation(async (name: string) => {
    if (name.endsWith('.GetSnapshot')) return initialSnapshot()
    if (name.endsWith('.SessionAlerts')) return [{ ...alert, bannerSeconds: 5 }]
  })
  render(<Overlay />)
  await act(async () => {})
  expect(screen.getByRole('button', { name: 'open BTCUSDT in terminal' })).toBeTruthy()
  await act(async () => {
    await vi.advanceTimersByTimeAsync(3000)
    backend.listeners.get('alert:new')?.({ data: { ...alert, id: 2, symbol: 'ETHUSDT', bannerSeconds: -1 } })
    await vi.advanceTimersByTimeAsync(100)
  })
  expect(screen.getByRole('button', { name: 'open ETHUSDT in terminal' })).toBeTruthy()
  await act(async () => { await vi.advanceTimersByTimeAsync(2400) })
  expect(screen.queryByRole('button', { name: 'open BTCUSDT in terminal' })).toBeNull()
  expect(screen.getByRole('button', { name: 'open ETHUSDT in terminal' })).toBeTruthy()
  await act(async () => { await vi.advanceTimersByTimeAsync(360000) })
  expect(screen.getByRole('button', { name: 'open ETHUSDT in terminal' })).toBeTruthy()
})

it('hides the native overlay when the last timed banner expires', async () => {
  vi.useFakeTimers()
  backend.call.mockImplementation(async (name: string) => {
    if (name.endsWith('.GetSnapshot')) return initialSnapshot()
    if (name.endsWith('.SessionAlerts')) return [{ ...alert, bannerSeconds: 15 }]
  })
  render(<Overlay />)
  await act(async () => {})
  await act(async () => { await vi.advanceTimersByTimeAsync(14999) })
  expect(screen.getByRole('button', { name: 'open BTCUSDT in terminal' })).toBeTruthy()
  await act(async () => { await vi.advanceTimersByTimeAsync(501) })
  expect(screen.queryByRole('button', { name: 'open BTCUSDT in terminal' })).toBeNull()
  expect(backend.call).toHaveBeenCalledWith('main.DesktopService.HideOverlay', alert.id)
})

it('edits turnover in millions while saving the full USD amount', async () => {
  render(<RuleDialog open onOpenChange={vi.fn()} rule={rule} sounds={['chime']} onSaved={vi.fn()} />)
  const input = screen.getByLabelText('min 24h turnover, million USD')
  expect(input.getAttribute('value')).toBe('80')
  fireEvent.change(input, { target: { value: '12.5' } })
  await userEvent.click(screen.getByRole('button', { name: 'save alert' }))
  await waitFor(() => expect(backend.call).toHaveBeenCalledWith('main.DesktopService.SaveRule', { ...rule, minQuoteVolume: 12_500_000 }))
})

it('uses an unused color when creating an alert from settings', async () => {
  render(<RulesSettings rules={[{ ...rule, color: '#fa423e' }]} sounds={['chime']} onSaved={vi.fn()} />)
  await userEvent.click(screen.getByRole('button', { name: 'new alert' }))
  expect(screen.getByRole('button', { name: 'alert color' }).style.backgroundColor).toBe('rgb(242, 153, 74)')
})

it('preserves the color of an existing alert', () => {
  render(<RuleDialog open onOpenChange={vi.fn()} rule={rule} rules={[rule]} sounds={['chime']} onSaved={vi.fn()} />)
  expect(screen.getByRole('button', { name: 'alert color' }).style.backgroundColor).toBe('rgb(65, 201, 119)')
})

it('reorders opening exchanges without changing their selection on click', async () => {
  render(<RuleDialog open onOpenChange={vi.fn()} rule={{ ...rule, kind: 'listing', listingTypes: ['spot'], listingExchanges: ['upbit'] }} sounds={['chime']} onSaved={vi.fn()} />)
  const bybit = screen.getByRole('button', { name: /Bybit, priority 2/ })
  await userEvent.click(bybit)
  expect(screen.getByRole('button', { name: /Bybit, priority 2/ })).toBeTruthy()
  fireEvent.keyDown(bybit, { key: 'ArrowLeft' })
  expect(screen.getByRole('button', { name: /Bybit, priority 1/ })).toBeTruthy()
  await userEvent.click(screen.getByRole('button', { name: 'save alert' }))
  await waitFor(() => expect(backend.call).toHaveBeenCalledWith('main.DesktopService.SaveRule', expect.objectContaining({ openExchanges: ['bybit', 'binance', 'okx', 'gate'] })))
})

it('lets the last exchange card become inactive and requests a selection before saving', async () => {
  render(<RuleDialog open onOpenChange={vi.fn()} rule={{ ...rule, kind: 'listing', listingTypes: ['spot'], listingExchanges: ['upbit'] }} sounds={['chime']} onSaved={vi.fn()} />)
  const upbit = screen.getByRole('checkbox', { name: 'Upbit' })
  await userEvent.click(upbit)
  expect(upbit.getAttribute('aria-checked')).toBe('false')
  expect(upbit.className).not.toContain('is-active')
  await userEvent.click(screen.getByRole('button', { name: 'save alert' }))
  expect(screen.getByText('choose at least one listing exchange')).toBeTruthy()
  expect(backend.call).not.toHaveBeenCalledWith('main.DesktopService.SaveRule', expect.anything())
})

it('uses the listing source icon and event in the banner title without time or link controls', async () => {
  applyLanguage('ru')
  backend.call.mockImplementation(async (name: string) => {
    if (name.endsWith('.GetSnapshot')) return { ...initialSnapshot(), settings: { ...initialSnapshot().settings, language: 'ru' } }
    if (name.endsWith('.SessionAlerts')) return [{ ...alert, symbol: 'REZUSDT', kind: 'listing', direction: '', listing: {
      exchange: 'upbit', asset: 'REZ', marketType: 'roadmap', markets: [], url: 'https://upbit.com/notice', detectedTimeUs: 1, sentTimeUs: 1,
    } }]
  })
  const { container } = render(<Overlay />)
  await screen.findByText(/Анонс листинга/)
  expect(container.querySelector('.toast-symbol')?.textContent).toBe('REZUSDT – Анонс листинга')
  expect(screen.getByRole('img', { name: 'Upbit' })).toBeTruthy()
  expect(screen.queryByRole('img', { name: 'Binance' })).toBeNull()
  expect(container.querySelector('time')).toBeNull()
  expect(container.querySelector('.listing-link')).toBeNull()
})
