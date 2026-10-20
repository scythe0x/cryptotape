import { useMemo, useRef, useState } from 'react'
import {
  flexRender, getCoreRowModel, getSortedRowModel, useReactTable,
  type ColumnDef, type SortingState,
} from '@tanstack/react-table'
import { useVirtualizer } from '@tanstack/react-virtual'
import { ArrowDownRight, ArrowUpRight } from 'lucide-react'
import type { Alert } from '../api'
import { desktop } from '../api'

type Props = {
  alerts: Alert[]
  search: string
  hasMore: boolean
  loadMore: () => void
  onError: (error: string) => void
}

export function AlertsTable({ alerts, search, hasMore, loadMore, onError }: Props) {
  const [sorting, setSorting] = useState<SortingState>([{ id: 'id', desc: true }])
  const scrollRef = useRef<HTMLDivElement>(null)
  const visible = useMemo(() => {
    const query = search.trim().toUpperCase()
    return query ? alerts.filter((alert) => alert.symbol.includes(query) || alert.ruleName.toUpperCase().includes(query)) : alerts
  }, [alerts, search])

  const columns = useMemo<ColumnDef<Alert>[]>(() => [
    { accessorKey: 'id', header: 'time', cell: ({ row }) => new Date(row.original.createdAtMs).toLocaleTimeString('en-GB', { hour12: false }), size: 100 },
    { accessorKey: 'symbol', header: 'market', cell: ({ getValue }) => <strong>{String(getValue())}</strong>, size: 180 },
    { accessorKey: 'changePercent', header: 'move', cell: ({ row }) => <span className={row.original.direction === 'down' ? 'negative' : 'positive'}>
      {row.original.direction === 'down' ? <ArrowDownRight size={14} /> : <ArrowUpRight size={14} />}
      {row.original.changePercent > 0 ? '+' : ''}{row.original.changePercent.toFixed(2)}%
    </span>, size: 115 },
    { accessorKey: 'ruleName', header: 'alert', cell: ({ getValue }) => String(getValue()), size: 180 },
    { accessorKey: 'terminalStatus', header: 'terminal', cell: ({ row }) => <span title={row.original.terminalError} className={`terminal-status terminal-${row.original.terminalStatus}`}>
      {row.original.terminalStatus === 'skipped' ? 'manual' : row.original.terminalStatus}
    </span>, size: 96 },
  ], [])

  const table = useReactTable({
    data: visible, columns, state: { sorting }, onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(), getSortedRowModel: getSortedRowModel(),
  })
  const rows = table.getRowModel().rows
  const virtualizer = useVirtualizer({
    count: rows.length, getScrollElement: () => scrollRef.current,
    estimateSize: () => 40, overscan: 8,
  })

  function onScroll() {
    const element = scrollRef.current
    if (element && hasMore && element.scrollHeight - element.scrollTop - element.clientHeight < 260) {
      loadMore()
    }
  }

  async function open(alert: Alert) {
    try {
      await desktop.openTerminal(alert.id, alert.symbol)
    } catch (error) {
      onError(`terminal: ${String(error)}`)
    }
  }

  return <section className="feed-panel">
    <div className="table-header">
      {table.getHeaderGroups().map((group) => group.headers.map((header) =>
        <button key={header.id} style={{ flex: `${header.getSize()} 0 0` }} onClick={header.column.getToggleSortingHandler()}>
          {flexRender(header.column.columnDef.header, header.getContext())}
        </button>,
      ))}
    </div>
    <div className="table-scroll" ref={scrollRef} onScroll={onScroll}>
      {rows.length === 0 ? <div className="empty-state">{search ? 'nothing found' : 'waiting for alerts'}</div>
        : <div className="virtual-body" style={{ height: virtualizer.getTotalSize() }}>
        {virtualizer.getVirtualItems().map((item) => {
          const row = rows[item.index]
          return <div key={row.id} className="table-row" style={{ transform: `translateY(${item.start}px)` }}
            onClick={() => void open(row.original)} title={`open ${row.original.symbol} in terminal`}>
            {row.getVisibleCells().map((cell) => <div key={cell.id} className="table-cell" style={{ flex: `${cell.column.getSize()} 0 0` }}>
              {flexRender(cell.column.columnDef.cell, cell.getContext())}
            </div>)}
          </div>
        })}
      </div>}
      {hasMore && <div className="more-row">scroll for older alerts</div>}
    </div>
  </section>
}
