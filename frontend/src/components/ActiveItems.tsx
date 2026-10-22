import { useId, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'
import { DragDropProvider, useDragOperation, useDroppable } from '@dnd-kit/react'
import { useSortable } from '@dnd-kit/react/sortable'
import { CollisionPriority } from '@dnd-kit/abstract'
import { move } from '@dnd-kit/helpers'
import './active-items.css'

export type ActiveItem<T extends string> = { id: T; label: string; content: ReactNode }
type ItemZoneID = 'active' | 'inactive'
type ItemDragData = { zone: ItemZoneID }
type Groups<T extends string> = { active: T[]; inactive: T[] }
type Props<T extends string> = {
  items: readonly ActiveItem<T>[]
  value: readonly T[]
  onChange: (value: T[]) => void
  activeLabel: string
  inactiveLabel: string
}

export function ActiveItems<T extends string>({ items, value, onChange, activeLabel, inactiveLabel }: Props<T>) {
  const scope = useId()
  const [draft, setDraft] = useState<Groups<T> | null>(null)
  const latest = useRef<Groups<T> | null>(null)
  const groups: Groups<T> = draft ?? {
    active: [...value],
    inactive: items.filter((item) => !value.includes(item.id)).map((item) => item.id),
  }
  const labels = { active: activeLabel, inactive: inactiveLabel }

  function onKeyMove(event: KeyboardEvent<HTMLButtonElement>, id: T, group: keyof Groups<T>) {
    if (latest.current || !['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key)) {
      return
    }
    event.preventDefault()
    const next = { active: [...groups.active], inactive: [...groups.inactive] }
    const source = next[group]
    const index = source.indexOf(id)

    if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
      const target = index + (event.key === 'ArrowLeft' ? -1 : 1)
      if (target < 0 || target >= source.length) return
      source.splice(index, 1)
      source.splice(target, 0, id)
    } else {
      const target = event.key === 'ArrowUp' ? 'active' : 'inactive'
      if (group === target) return
      source.splice(index, 1)
      next[target].push(id)
    }
    if (!sameItems(next.active, value)) onChange(next.active)
  }

  return (
    <DragDropProvider
      onDragStart={() => {
        latest.current = groups
        setDraft(groups)
      }}
      onDragOver={(event) => {
        const next = move(latest.current ?? groups, event)
        latest.current = next
        setDraft(next)
      }}
      onDragEnd={(event) => {
        const next = latest.current
        latest.current = null
        setDraft(null)
        if (!event.canceled && event.operation.target && next && !sameItems(next.active, value)) {
          onChange(next.active)
        }
      }}
    >
      <div className="active-items">
        {(['active', 'inactive'] as const).map((group) => (
          <ItemZone key={group} id={group} itemIds={groups[group]} label={labels[group]}>
            {groups[group].map((id, index) => {
              const item = items.find((candidate) => candidate.id === id)
              return item ? (
                <SortableItem
                  key={id}
                  item={item}
                  index={index}
                  group={group}
                  scope={scope}
                  onKeyDown={(event) => onKeyMove(event, id, group)}
                />
              ) : null
            })}
          </ItemZone>
        ))}
      </div>
    </DragDropProvider>
  )
}

function sameItems<T>(left: readonly T[], right: readonly T[]) {
  return left.length === right.length && left.every((item, index) => item === right[index])
}

function ItemZone({ id, itemIds, label, children }: { id: ItemZoneID; itemIds: readonly string[]; label: string; children: ReactNode }) {
  const { ref } = useDroppable<ItemDragData>({ id, data: { zone: id }, collisionPriority: CollisionPriority.Low })
  const { source, target } = useDragOperation<ItemDragData>()
  const isDropTarget = Boolean(source && target && (
    target.id === source.id
      ? itemIds.some((itemId) => itemId === source.id)
      : target.data.zone === id
  ))
  return (
    <section className={`active-items-zone active-items-${id}${isDropTarget ? ' is-drop-target' : ''}`}>
      <span className="active-items-label">{label}</span>
      <div ref={ref} className="active-items-list" role="group" aria-label={label}>
        {children}
      </div>
    </section>
  )
}

function SortableItem<T extends string>({ item, index, group, scope, onKeyDown }: {
  item: ActiveItem<T>
  index: number
  group: ItemZoneID
  scope: string
  onKeyDown: (event: KeyboardEvent<HTMLButtonElement>) => void
}) {
  const { ref, isDragging } = useSortable<ItemDragData>({ id: item.id, index, group, data: { zone: group }, type: scope, accept: scope })
  return (
    <button
      ref={ref}
      type="button"
      className={`active-items-item${isDragging ? ' is-dragging' : ''}`}
      aria-label={item.label}
      aria-keyshortcuts="ArrowLeft ArrowRight ArrowUp ArrowDown"
      title={item.label}
      onKeyDown={onKeyDown}
    >
      {item.content}
    </button>
  )
}
