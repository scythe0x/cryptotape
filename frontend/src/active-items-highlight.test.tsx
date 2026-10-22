// @vitest-environment jsdom
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { act, cleanup, render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { ActiveItems } from './components/ActiveItems'

type Zone = 'active' | 'inactive'
type DragEntity = { id: string; data: { zone: Zone } }
type DragEnd = { canceled: boolean; operation: { target: DragEntity | null } }
type DragHandlers = { onDragStart?: () => void; onDragEnd?: (event: DragEnd) => void }
const monitor = vi.hoisted(() => {
  const operation: { source: DragEntity | null; target: DragEntity | null } = { source: null, target: null }
  const handlers: DragHandlers = {}
  return { operation, handlers, droppable: vi.fn(), sortable: vi.fn() }
})

vi.mock('@dnd-kit/react', () => ({
  DragDropProvider: ({ children, onDragStart, onDragEnd }: DragHandlers & { children: ReactNode }) => {
    monitor.handlers.onDragStart = onDragStart
    monitor.handlers.onDragEnd = onDragEnd
    return children
  },
  useDragOperation: () => monitor.operation,
  useDroppable: (input: { id: string; data: { zone: Zone } }) => {
    monitor.droppable(input)
    return { ref: vi.fn() }
  },
}))
vi.mock('@dnd-kit/react/sortable', () => ({
  useSortable: (input: { id: string; data: { zone: Zone } }) => {
    monitor.sortable(input)
    return { ref: vi.fn(), isDragging: false }
  },
}))

const items = [
  { id: 'one', label: 'first', content: 'first' },
  { id: 'two', label: 'second', content: 'second' },
]
const source: DragEntity = { id: 'one', data: { zone: 'active' } }
const props = { items, value: ['one'], onChange: vi.fn(), activeLabel: 'active', inactiveLabel: 'inactive' }

function zone(name: Zone) {
  const section = screen.getByRole('group', { name }).closest('section')
  if (!section) throw new Error('missing item zone')
  return section
}

beforeEach(() => {
  monitor.operation = { source: null, target: null }
  monitor.droppable.mockClear()
  monitor.sortable.mockClear()
  props.onChange.mockClear()
})
afterEach(cleanup)

it('tags containers and sortable children with their current zone', () => {
  const { rerender } = render(<ActiveItems {...props} />)
  expect(monitor.droppable).toHaveBeenCalledWith(expect.objectContaining({ id: 'active', data: { zone: 'active' } }))
  expect(monitor.droppable).toHaveBeenCalledWith(expect.objectContaining({ id: 'inactive', data: { zone: 'inactive' } }))
  expect(monitor.sortable).toHaveBeenCalledWith(expect.objectContaining({ id: 'one', data: { zone: 'active' } }))
  expect(monitor.sortable).toHaveBeenCalledWith(expect.objectContaining({ id: 'two', data: { zone: 'inactive' } }))
  rerender(<ActiveItems {...props} value={[]} />)
  expect(monitor.sortable).toHaveBeenLastCalledWith(expect.objectContaining({ id: 'two', data: { zone: 'inactive' } }))
  expect(monitor.sortable).toHaveBeenCalledWith(expect.objectContaining({ id: 'one', data: { zone: 'inactive' } }))
})

it('keeps the whole zone highlighted over its container, child, and moved drag source', () => {
  monitor.operation = { source, target: { id: 'inactive', data: { zone: 'inactive' } } }
  const { rerender } = render(<ActiveItems {...props} />)
  expect(zone('inactive').classList.contains('is-drop-target')).toBe(true)
  expect(zone('active').classList.contains('is-drop-target')).toBe(false)
  monitor.operation = { source, target: { id: 'two', data: { zone: 'inactive' } } }
  rerender(<ActiveItems {...props} />)
  expect(zone('inactive').classList.contains('is-drop-target')).toBe(true)
  // sortable can target the moved source before its zone metadata catches up.
  const movedSource: DragEntity = { id: 'one', data: { zone: 'active' } }
  monitor.operation = { source: movedSource, target: movedSource }
  rerender(<ActiveItems {...props} value={[]} />)
  expect(zone('inactive').classList.contains('is-drop-target')).toBe(true)
  expect(zone('active').classList.contains('is-drop-target')).toBe(false)
  expect(props.onChange).not.toHaveBeenCalled()
})

it('clears highlighting outside a target and after cancellation clears the drag source', () => {
  monitor.operation = { source, target: { id: 'two', data: { zone: 'inactive' } } }
  const { rerender } = render(<ActiveItems {...props} />)
  expect(zone('inactive').classList.contains('is-drop-target')).toBe(true)
  monitor.operation = { source, target: null }
  rerender(<ActiveItems {...props} />)
  expect(zone('inactive').classList.contains('is-drop-target')).toBe(false)
  act(() => monitor.handlers.onDragStart?.())
  monitor.operation = { source: null, target: { id: 'two', data: { zone: 'inactive' } } }
  act(() => monitor.handlers.onDragEnd?.({ canceled: true, operation: { target: monitor.operation.target } }))
  rerender(<ActiveItems {...props} />)
  expect(zone('inactive').classList.contains('is-drop-target')).toBe(false)
  expect(zone('active').classList.contains('is-drop-target')).toBe(false)
  expect(props.onChange).not.toHaveBeenCalled()
})
