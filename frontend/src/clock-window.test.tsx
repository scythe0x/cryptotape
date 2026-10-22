// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { ClockWindow } from './ClockWindow'
import { applyLanguage } from './i18n'

const api = vi.hoisted(() => ({ clockTimezone: vi.fn(), setClockTimezone: vi.fn(), snapshot: vi.fn(), close: vi.fn() }))
vi.mock('./api', () => ({ desktop: api }))
vi.mock('@wailsio/runtime', () => ({ Window: { Close: api.close } }))
vi.mock('./useClockTime', () => ({ useClockTime: () => Date.parse('2026-10-21T23:59:55Z') }))

beforeEach(() => {
  applyLanguage('en')
  api.clockTimezone.mockResolvedValue('UTC300')
  api.setClockTimezone.mockResolvedValue(undefined)
  api.snapshot.mockResolvedValue({ settings: { language: 'en' } })
  api.close.mockResolvedValue(undefined)
})
afterEach(() => { cleanup(); vi.clearAllMocks() })

it('restores the timezone, saves changes and keeps UTC candle warnings', async () => {
  render(<ClockWindow />)
  const select = await screen.findByRole<HTMLSelectElement>('combobox', { name: 'timezone' })
  await waitFor(() => expect(select.value).toBe('UTC300'))
  expect(screen.getByRole('img', { name: '04:59:55' })).toBeTruthy()
  expect(screen.getByText('1 DAY')).toBeTruthy()
  fireEvent.change(select, { target: { value: 'UTC0' } })
  await waitFor(() => expect(select.value).toBe('UTC0'))
  expect(api.setClockTimezone).toHaveBeenCalledWith('UTC0')
  expect(screen.getByRole('img', { name: '23:59:55' })).toBeTruthy()
  expect(screen.getByText('1 DAY')).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: 'close' }))
  expect(api.close).toHaveBeenCalledOnce()
})

it('retains the saved timezone when persistence fails', async () => {
  api.setClockTimezone.mockRejectedValueOnce(new Error('disk unavailable'))
  render(<ClockWindow />)
  const select = await screen.findByRole<HTMLSelectElement>('combobox', { name: 'timezone' })
  await waitFor(() => expect(select.value).toBe('UTC300'))
  fireEvent.change(select, { target: { value: 'UTC0' } })
  await screen.findByTitle('Error: disk unavailable')
  expect(select.value).toBe('UTC300')
})
