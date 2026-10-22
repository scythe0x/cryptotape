import { useEffect, type RefObject } from 'react'
import { desktop } from './api'
import { minimumTableRowHeight, tableColumnIDs, tableMinimumWidth, type TableColumnID } from './table-columns'

type Minimum = { width: number; height: number; viewportWidth: number; viewportHeight: number }

function pixels(value: string): number {
  return Number.parseFloat(value) || 0
}

export function measureMainWindowMinimum(shell: HTMLElement, columns: readonly TableColumnID[]): Minimum | null {
  const toolbar = shell.querySelector<HTMLElement>('[data-window-toolbar]')
  const viewport = shell.querySelector<HTMLElement>('.table-scroll')
  if (!toolbar || !viewport || shell.clientHeight === 0) return null

  const style = window.getComputedStyle(toolbar)
  const children = [...toolbar.children].filter((element) => window.getComputedStyle(element).display !== 'none')
  const contentWidth = children.reduce((width, element) => width + element.getBoundingClientRect().width, 0)
  const toolbarWidth = contentWidth + Math.max(0, children.length - 1) * pixels(style.columnGap)
    + pixels(style.paddingLeft) + pixels(style.paddingRight)
    + pixels(style.borderLeftWidth) + pixels(style.borderRightWidth)
  const chromeHeight = shell.clientHeight - viewport.clientHeight

  return {
    width: Math.ceil(Math.max(1, tableMinimumWidth(columns), toolbarWidth)),
    height: Math.ceil(Math.max(1, chromeHeight + (columns.length ? minimumTableRowHeight : 0))),
    viewportWidth: window.innerWidth,
    viewportHeight: window.innerHeight,
  }
}

export function useMainWindowMinimum(
  shellRef: RefObject<HTMLElement | null>,
  columns: readonly TableColumnID[],
  onError: (message: string) => void,
) {
  const columnsKey = columns.join(',')

  useEffect(() => {
    const element = shellRef.current
    if (!element) return
    const shell: HTMLElement = element
    let frame = 0
    let stopped = false
    let previous = ''
    const visibleColumns = columnsKey ? columnsKey.split(',').filter(isTableColumn) : []

    function update() {
      frame = 0
      if (stopped) return
      const minimum = measureMainWindowMinimum(shell, visibleColumns)
      if (!minimum) return
      const key = JSON.stringify(minimum)
      if (key === previous) return
      previous = key
      void desktop.setMainWindowMinimum(minimum.width, minimum.height, minimum.viewportWidth, minimum.viewportHeight)
        .catch((error: unknown) => {
          if (stopped) return
          previous = ''
          onError(String(error))
        })
    }

    function schedule() {
      if (!frame) frame = window.requestAnimationFrame(update)
    }

    const resizeObserver = new ResizeObserver(schedule)
    function observeLayout() {
      resizeObserver.disconnect()
      resizeObserver.observe(shell)
      for (const child of shell.children) resizeObserver.observe(child)
      const toolbar = shell.querySelector('[data-window-toolbar]')
      if (toolbar) {
        resizeObserver.observe(toolbar)
        for (const child of toolbar.children) resizeObserver.observe(child)
      }
      const viewport = shell.querySelector('.table-scroll')
      if (viewport) resizeObserver.observe(viewport)
      schedule()
    }

    const mutationObserver = new MutationObserver(observeLayout)
    mutationObserver.observe(shell, { childList: true })
    const toolbar = shell.querySelector('[data-window-toolbar]')
    if (toolbar) mutationObserver.observe(toolbar, { childList: true, subtree: true, characterData: true })
    window.addEventListener('resize', schedule)
    observeLayout()
    void document.fonts?.ready.then(() => { if (!stopped) schedule() })
    return () => {
      stopped = true
      window.cancelAnimationFrame(frame)
      resizeObserver.disconnect()
      mutationObserver.disconnect()
      window.removeEventListener('resize', schedule)
    }
  }, [shellRef, columnsKey, onError])
}

function isTableColumn(value: string): value is TableColumnID {
  return tableColumnIDs.some((column) => column === value)
}
