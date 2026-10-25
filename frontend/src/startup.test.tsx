// @vitest-environment jsdom
import { afterEach, expect, it, vi } from 'vitest'
import { act, screen } from '@testing-library/react'
import type { Root } from 'react-dom/client'

const startup = vi.hoisted(() => ({ root: undefined as Root | undefined }))

vi.mock('react-dom/client', async (importOriginal) => {
  const client = await importOriginal<typeof import('react-dom/client')>()
  return {
    ...client,
    createRoot: (...args: Parameters<typeof client.createRoot>) => {
      startup.root = client.createRoot(...args)
      return startup.root
    },
  }
})
vi.mock('./App', () => ({ App: () => <p>main view</p> }))
vi.mock('./SettingsWindow', () => ({ SettingsWindow: () => <p>settings view</p> }))
vi.mock('./Overlay', () => ({ Overlay: () => <p>overlay view</p> }))
vi.mock('./ClockWindow', () => ({ ClockWindow: () => <p>clock view</p> }))

// native calls are unavailable until a view explicitly needs application data.
vi.mock('@wailsio/runtime', () => {
  throw new Error('startup must not require native authentication')
})

afterEach(async () => {
  await act(async () => { startup.root?.unmount() })
  startup.root = undefined
  document.body.replaceChildren()
  window.history.replaceState(null, '', '/')
  vi.resetModules()
})

it.each([
  ['/', 'main view'],
  ['/?activation=1', 'main view'],
  ['/?settings=1', 'settings view'],
  ['/?overlay=1', 'overlay view'],
  ['/?clock=1', 'clock view'],
])('opens %s immediately without an account or key', async (url, view) => {
  window.history.replaceState(null, '', url)
  const container = document.createElement('div')
  container.id = 'root'
  document.body.append(container)

  await act(async () => { await import('./main') })

  expect(await screen.findByText(view)).toBeTruthy()
})
