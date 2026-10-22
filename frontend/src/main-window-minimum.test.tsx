// @vitest-environment jsdom
import { act, cleanup, render, waitFor } from '@testing-library/react'
import { useRef } from 'react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { measureMainWindowMinimum, useMainWindowMinimum } from './useMainWindowMinimum'
import { tableColumnIDs, tableGridTemplate, tableMinimumWidth, type TableColumnID } from './table-columns'

const backend = vi.hoisted(() => ({ setMinimum: vi.fn().mockResolvedValue(undefined) }))
vi.mock('./api', () => ({ desktop: { setMainWindowMinimum: backend.setMinimum } }))

const resizeObservers = new Set<TestResizeObserver>()
class TestResizeObserver implements ResizeObserver {
  constructor(readonly callback: ResizeObserverCallback) { resizeObservers.add(this) }
  observe() {}
  unobserve() {}
  disconnect() {}
}
beforeEach(() => {
  backend.setMinimum.mockReset().mockResolvedValue(undefined)
  resizeObservers.clear()
  vi.stubGlobal('ResizeObserver', TestResizeObserver)
})
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.unstubAllGlobals() })

function dimensions(element: HTMLElement, width: number, height: number) {
  Object.defineProperties(element, {
    clientWidth: { configurable: true, value: width },
    clientHeight: { configurable: true, value: height },
  })
}

function layout() {
  const shell = document.createElement('main')
  shell.innerHTML = '<header data-window-toolbar style="padding:0 10px 0 14px;column-gap:8px;border:0"><div></div><div></div></header><div class="table-scroll"></div>'
  const toolbar = shell.querySelector<HTMLElement>('header')
  const viewport = shell.querySelector<HTMLElement>('.table-scroll')
  if (!toolbar || !viewport) throw new Error('missing layout element')
  dimensions(shell, 440, 740)
  dimensions(viewport, 440, 670)
  const status = toolbar.children[0]
  const actions = toolbar.children[1]
  vi.spyOn(status, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 70, 40))
  vi.spyOn(actions, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 106, 40))
  return { shell, toolbar, viewport, status, actions }
}

it('shares the exact grid minima with native window sizing', () => {
  expect(tableMinimumWidth(tableColumnIDs)).toBe(312)
  expect(tableMinimumWidth(['event', 'time'])).toBe(150)
  expect(tableMinimumWidth([])).toBe(0)
  expect(tableGridTemplate(['terminals', 'event', 'ticker'])).toBe('72px minmax(84px, 1fr) minmax(90px, 120px)')
})

it('takes the larger of table minima and intrinsic toolbar groups', () => {
  const { shell, status } = layout()
  expect(measureMainWindowMinimum(shell, tableColumnIDs)).toMatchObject({ width: 312, height: 114 })
  vi.mocked(status.getBoundingClientRect).mockReturnValue(new DOMRect(0, 0, 240, 40))
  expect(measureMainWindowMinimum(shell, tableColumnIDs)).toMatchObject({ width: 378, height: 114 })
})

it('lowers the minimum with hidden columns and omits both table header and row when all hidden', () => {
  const { shell, viewport } = layout()
  expect(measureMainWindowMinimum(shell, ['event'])).toMatchObject({ width: 208, height: 114 })
  dimensions(viewport, 440, 700)
  expect(measureMainWindowMinimum(shell, [])).toMatchObject({ width: 208, height: 40 })
})

it('includes extra visible panels in the vertical minimum without measuring feed contents', () => {
  const { shell, viewport } = layout()
  dimensions(viewport, 440, 500)
  expect(measureMainWindowMinimum(shell, tableColumnIDs)?.height).toBe(284)
})

function HookHarness({ columns, onError }: { columns: readonly TableColumnID[]; onError: (message: string) => void }) {
  const shell = useRef<HTMLElement>(null)
  useMainWindowMinimum(shell, columns, onError)
  return <main ref={shell}><header data-window-toolbar><div>status</div><div>actions</div></header><div className="table-scroll" /></main>
}

it('updates native constraints on observed layout changes and deduplicates unchanged sizes', async () => {
  let viewportHeight = 670
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockImplementation(function (this: HTMLElement) {
    return this.classList.contains('table-scroll') ? viewportHeight : 740
  })
  const onError = vi.fn()
  const { rerender } = render(<HookHarness columns={tableColumnIDs} onError={onError} />)
  await waitFor(() => expect(backend.setMinimum).toHaveBeenCalledTimes(1))
  expect(backend.setMinimum).toHaveBeenLastCalledWith(312, 114, window.innerWidth, window.innerHeight)
  act(() => window.dispatchEvent(new Event('resize')))
  await new Promise((resolve) => window.setTimeout(resolve, 30))
  expect(backend.setMinimum).toHaveBeenCalledTimes(1)
  viewportHeight = 600
  act(() => {
    for (const observer of resizeObservers) observer.callback([], observer)
  })
  await waitFor(() => expect(backend.setMinimum).toHaveBeenCalledTimes(2))
  expect(backend.setMinimum).toHaveBeenLastCalledWith(312, 184, window.innerWidth, window.innerHeight)
  viewportHeight = 700
  rerender(<HookHarness columns={[]} onError={onError} />)
  await waitFor(() => expect(backend.setMinimum).toHaveBeenCalledTimes(3))
  expect(backend.setMinimum).toHaveBeenLastCalledWith(1, 40, window.innerWidth, window.innerHeight)
})

it('surfaces native sizing errors', async () => {
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(100)
  backend.setMinimum.mockRejectedValueOnce(new Error('native failed'))
  const onError = vi.fn()
  render(<HookHarness columns={tableColumnIDs} onError={onError} />)
  await waitFor(() => expect(onError).toHaveBeenCalledWith('Error: native failed'))
})
