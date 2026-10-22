// @vitest-environment jsdom
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { LicenseBoundary } from './LicenseBoundary'
import { Activation } from './Activation'

const backend = vi.hoisted(() => ({ call: vi.fn<(name: string, ...args: unknown[]) => Promise<unknown>>(), listener: undefined as ((event: { data: unknown }) => void) | undefined }))
vi.mock('@wailsio/runtime', () => ({
  Call: { ByName: backend.call },
  Browser: { OpenURL: vi.fn() },
  Events: { On: (_name: string, listener: (event: { data: unknown }) => void) => { backend.listener = listener; return () => { backend.listener = undefined } } },
}))
const state = (phase: string, message = '', canRetry = false) => ({ phase, code: '', message, botUrl: 'https://t.me/mythrascalping_bot', canRetry })
beforeEach(() => { backend.call.mockReset(); backend.listener = undefined })
afterEach(() => { cleanup(); vi.useRealTimers() })

it('keeps useful views unmounted until authorized and removes them on revocation', async () => {
  backend.call.mockResolvedValue(state('locked'))
  render(<LicenseBoundary><p>private application</p></LicenseBoundary>)
  expect(screen.queryByText('private application')).toBeNull()
  await act(async () => { backend.listener?.({ data: state('ready') }) })
  expect(screen.getByText('private application')).toBeTruthy()
  await act(async () => { backend.listener?.({ data: state('locked') }) })
  expect(screen.queryByText('private application')).toBeNull()
})
it('does not let a late startup snapshot override revocation', async () => {
  let resolve: ((value: unknown) => void) | undefined
  backend.call.mockReturnValue(new Promise<unknown>((done) => { resolve = done }))
  render(<LicenseBoundary><p>private application</p></LicenseBoundary>)
  await act(async () => { backend.listener?.({ data: state('locked') }); resolve?.(state('ready')) })
  expect(screen.queryByText('private application')).toBeNull()
})
it('shows short errors and prevents duplicate activation submissions', async () => {
  vi.useFakeTimers()
  let resolve: ((value: unknown) => void) | undefined
  backend.call.mockImplementation((name: string) => {
    if (name.endsWith('.Activate')) return new Promise<unknown>((done) => { resolve = done })
    return Promise.resolve(state('locked'))
  })
  render(<Activation />)
  await act(async () => { backend.listener?.({ data: state('locked') }) })
  fireEvent.change(screen.getByLabelText('Ключ доступа'), { target: { value: 'test-key' } })
  const submit = screen.getByRole('button', { name: 'Войти' })
  fireEvent.click(submit)
  fireEvent.click(submit)
  expect(backend.call.mock.calls.filter(([name]) => name.endsWith('.Activate')).length).toBe(1)
  await act(async () => { resolve?.(state('locked', 'Неверный ключ')) })
  expect(screen.getByRole('alert').textContent).toBe('Неверный ключ')
})
it('can retry a saved key without exposing it to the form', async () => {
  backend.call.mockResolvedValue(state('locked', 'Сервер недоступен', true))
  render(<Activation />)
  await act(async () => { backend.listener?.({ data: state('locked', 'Сервер недоступен', true) }) })
  fireEvent.click(screen.getByRole('button', { name: 'Повторить' }))
  await waitFor(() => expect(backend.call).toHaveBeenCalledWith('main.ActivationService.Retry'))
  expect(screen.getByLabelText('Ключ доступа').getAttribute('value')).toBe('')
})

it('keeps activation errors when an earlier state poll finishes late', async () => {
  vi.useFakeTimers()
  let finishPoll: ((value: unknown) => void) | undefined
  backend.call.mockImplementation((name: string) => {
    if (name.endsWith('.State')) return new Promise<unknown>((done) => { finishPoll = done })
    return Promise.resolve(state('locked', 'Неверный ключ'))
  })
  render(<Activation />)
  await act(async () => { backend.listener?.({ data: state('locked') }) })
  await act(async () => { vi.advanceTimersByTime(250) })
  fireEvent.change(screen.getByLabelText('Ключ доступа'), { target: { value: 'test-key' } })
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Войти' })) })
  expect(screen.getByRole('alert').textContent).toBe('Неверный ключ')
  const resolvePoll = finishPoll
  if (!resolvePoll) throw new Error('state poll did not start')
  await act(async () => { resolvePoll(state('locked')) })
  expect(screen.getByRole('alert').textContent).toBe('Неверный ключ')
})

it('does not let an earlier poll override a newer license event', async () => {
  vi.useFakeTimers()
  let finishPoll: ((value: unknown) => void) | undefined
  backend.call.mockImplementation(() => new Promise<unknown>((done) => { finishPoll = done }))
  render(<Activation />)
  await act(async () => { vi.advanceTimersByTime(250) })
  const resolvePoll = finishPoll
  if (!resolvePoll) throw new Error('state poll did not start')
  await act(async () => {
    backend.listener?.({ data: state('locked', 'Ключ отозван') })
    resolvePoll(state('ready'))
  })
  expect(screen.getByRole('alert').textContent).toBe('Ключ отозван')
  expect(backend.call).not.toHaveBeenCalledWith('main.ActivationService.Continue')
})
