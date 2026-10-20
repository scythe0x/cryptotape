// @vitest-environment jsdom
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { act, cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { applyLanguage } from './i18n'
import type { Alert, Rule, Snapshot } from './api'
import { Overlay } from './Overlay'
import { RuleDialog } from './components/RuleDialog'
import { RulesSettings } from './components/RulesSettings'
import { ConnectionStatus } from './components/ConnectionStatus'
import { ProxySettings } from './components/ProxySettings'

const backend = vi.hoisted(() => ({
  call: vi.fn(),
  listeners: new Map<string, (event: { data: unknown }) => void>(),
}))
vi.mock('@wailsio/runtime', () => ({
  Call: { ByName: backend.call },
  Clipboard: { SetText: vi.fn() },
  Events: {
    On: (name: string, callback: (event: { data: unknown }) => void) => {
      backend.listeners.set(name, callback)
      return () => backend.listeners.delete(name)
    },
  },
}))

const rule: Rule = {
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
    cooldownSeconds: 60,
    monitorId: '',
    tigerPort: 7819,
    tigerLinkGroup: 'A',
  },
  health: {
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
  await userEvent.clear(screen.getByLabelText('min 24h turnover, USD'))
  await userEvent.type(screen.getByLabelText('min 24h turnover, USD'), '120000000')
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

it('opens the help tooltip on hover without a native title', async () => {
  const { HelpTip } = await import('./components/HelpTip')
  render(<HelpTip label="cooldown help">same ticker and rule wait 60 seconds</HelpTip>)
  const trigger = screen.getByRole('button', { name: 'cooldown help' })
  expect(trigger.getAttribute('title')).toBeNull()
  await userEvent.hover(trigger)
  expect((await screen.findByRole('tooltip')).textContent).toContain('same ticker and rule wait 60 seconds')
})

it('saves the language preference in settings', async () => {
  const { SettingsDialog } = await import('./components/SettingsDialog')
  const settings = {
    language: 'en' as const,
    proxies: [],
    activeProxyId: '',
    cooldownSeconds: 60,
    monitorId: '',
    tigerPort: 7819,
    tigerLinkGroup: 'A',
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
  expect(screen.getByRole('option', { name: 'перезвон' }).getAttribute('value')).toBe('chime')
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
  expect(screen.queryByRole('slider')).toBeNull()
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
  expect(screen.getByRole('status').textContent).toBe('подключено50 мс')
  expect(screen.queryByText('прогрев')).toBeNull()
  expect(screen.getByRole('status').className).toContain('latency-good')
})

it('creates a new rule through the full settings form', async () => {
  const user = userEvent.setup()
  const saved = vi.fn()
  render(<RulesSettings rules={[]} sounds={['chime', 'alarm']} onSaved={saved} />)
  await user.click(screen.getByRole('button', { name: 'new alert' }))
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
    name: 'fast knife',
    percent: 10,
    windowSeconds: 300,
    sound: 'alarm',
    color: '#a78bfa',
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
