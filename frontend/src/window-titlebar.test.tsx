// @vitest-environment jsdom
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { WindowTitleBar } from './components/WindowTitleBar'
import { applyLanguage } from './i18n'

const runtime = vi.hoisted(() => ({
  IsMaximised: vi.fn(),
  Minimise: vi.fn(),
  ToggleMaximise: vi.fn(),
  Close: vi.fn(),
}))
vi.mock('@wailsio/runtime', () => ({ Window: runtime }))

beforeEach(() => {
  applyLanguage('en')
  runtime.IsMaximised.mockResolvedValue(false)
  runtime.Minimise.mockResolvedValue(undefined)
  runtime.ToggleMaximise.mockResolvedValue(undefined)
  runtime.Close.mockResolvedValue(undefined)
})
afterEach(() => { cleanup(); vi.clearAllMocks() })

it('renders the brand, status and actions with working window controls', async () => {
  render(<WindowTitleBar status={<span>42 ms</span>} actions={<button>settings</button>} onError={vi.fn()} />)
  expect(screen.getByText('Mythra')).toBeTruthy()
  expect(screen.getByText('42 ms')).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: 'minimise' }))
  fireEvent.click(screen.getByRole('button', { name: 'maximise' }))
  fireEvent.click(screen.getByRole('button', { name: 'close' }))
  expect(runtime.Minimise).toHaveBeenCalledOnce()
  expect(runtime.ToggleMaximise).toHaveBeenCalledOnce()
  expect(runtime.Close).toHaveBeenCalledOnce()
  runtime.IsMaximised.mockResolvedValue(true)
  fireEvent(window, new Event('resize'))
  await screen.findByRole('button', { name: 'restore' })
})

it('uses the settings close handler even before content has loaded', () => {
  const close = vi.fn()
  render(<WindowTitleBar title="settings" onClose={close} onError={vi.fn()} />)
  fireEvent.click(screen.getByRole('button', { name: 'close' }))
  expect(close).toHaveBeenCalledOnce()
  expect(runtime.Close).not.toHaveBeenCalled()
})

it('reports window control failures', async () => {
  const onError = vi.fn()
  runtime.Minimise.mockRejectedValueOnce(new Error('unavailable'))
  render(<WindowTitleBar onError={onError} />)
  fireEvent.click(screen.getByRole('button', { name: 'minimise' }))
  await waitFor(() => expect(onError).toHaveBeenCalledWith('Error: unavailable'))
})
