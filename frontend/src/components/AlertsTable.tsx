import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type SortingState,
} from '@tanstack/react-table'
import { useVirtualizer } from '@tanstack/react-virtual'
import { ArrowUp, Check } from 'lucide-react'
import type { Alert, Rule, TerminalID } from '../api'
import { desktop } from '../api'
import { useTranslation } from '../i18n'
import { localizeDiagnostic } from '../i18n/diagnostics'
import { Ticker } from './Ticker'
import { AlertBadge } from './AlertBadge'
import { ExchangeIcon } from './ExchangeIcon'
import { TerminalButton, terminals } from './TerminalButton'

type Props = {
  alerts: Alert[]
  rules: Rule[]
  hasMore: boolean
  loadMore: () => void
  onError: (error: string) => void
}

export function AlertsTable({ alerts, rules, hasMore, loadMore, onError }: Props) {
  const { t } = useTranslation()
  const [sorting, setSorting] = useState<SortingState>([{ id: 'id', desc: true }])
  const [pausedAt, setPausedAt] = useState<number | null>(null)
  const scrollRef = useRef<HTMLDivElement>(null)
  const [copied, setCopied] = useState('')
  useEffect(() => {
    if (!copied) return
    const timer = window.setTimeout(() => setCopied(''), 1200)
    return () => window.clearTimeout(timer)
  }, [copied])

  const copy = useCallback(
    async (symbol: string) => {
      try {
        await desktop.copySymbol(symbol)
        setCopied(symbol)
      } catch (error) {
        onError(String(error))
      }
    },
    [onError],
  )
  const open = useCallback(
    async (alert: Alert, terminal: TerminalID) => {
      try {
        await desktop.openTerminal(alert.id, alert.symbol, terminal)
      } catch (error) {
        onError(`terminal: ${String(error)}`)
      }
    },
    [onError],
  )
  const colors = useMemo(() => new Map(rules.map((rule) => [rule.id, rule.color])), [rules])
  const columns = useMemo<ColumnDef<Alert>[]>(
    () => [
      {
        accessorKey: 'id',
        header: t('time'),
        cell: ({ row }) =>
          new Date(row.original.createdAtMs).toLocaleTimeString('en-GB', {
            hour12: false,
          }),
        size: 70,
      },
      {
        accessorKey: 'symbol',
        header: t('ticker'),
        cell: ({ row }) => (
          <button
            className="ticker-button"
            title={t('copySymbol', { symbol: row.original.symbol })}
            onClick={() => void copy(row.original.symbol)}
          >
            {copied === row.original.symbol ? <Check size={12} /> : <ExchangeIcon />}
            <strong>
              <Ticker symbol={row.original.symbol} />
            </strong>
          </button>
        ),
        size: 135,
      },
      {
        accessorKey: 'changePercent',
        header: t('move'),
        cell: ({ row }) => (
          <span className={row.original.direction === 'down' ? 'negative' : 'positive'}>
            {row.original.changePercent > 0 ? '+' : ''}
            {row.original.changePercent.toFixed(2)}%
          </span>
        ),
        size: 80,
      },
      {
        accessorKey: 'ruleName',
        header: t('alert'),
        cell: ({ row }) => (
          <AlertBadge
            name={row.original.ruleName}
            color={colors.get(row.original.ruleId) ?? row.original.color}
          />
        ),
        size: 95,
      },
      {
        id: 'terminals',
        header: '',
        enableSorting: false,
        cell: ({ row }) => (
          <div className="feed-terminals">
            {terminals.map(({ id }) => (
              <TerminalButton key={id} id={id} onClick={() => void open(row.original, id)} />
            ))}
          </div>
        ),
        size: 60,
      },
    ],
    [colors, copy, copied, open, t],
  )

  const gridTemplateColumns = columns.map((column) => `minmax(0, ${column.size ?? 150}fr)`).join(' ')

  const table = useReactTable({
    data: alerts,
    columns,
    state: { sorting },
    onSortingChange: (updater) => {
      setSorting(updater)
      setPausedAt(null)
      scrollRef.current?.scrollTo({ top: 0 })
    },
    getRowId: (alert) => String(alert.id),
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  })
  const rows = table.getRowModel().rows
  const getItemKey = useCallback((index: number) => rows[index].id, [rows])
  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => 40,
    overscan: 8,
    getItemKey,
    anchorTo: pausedAt === null ? 'start' : 'end',
  })
  const unseen = pausedAt === null ? 0 : alerts.filter((alert) => alert.id > pausedAt).length

  function onScroll() {
    const element = scrollRef.current
    if (element) {
      if (element.scrollTop <= 8) setPausedAt(null)
      else setPausedAt((current) => current ?? alerts[0]?.id ?? 0)
    }
    if (element && hasMore && element.scrollHeight - element.scrollTop - element.clientHeight < 260) {
      loadMore()
    }
  }

  return (
    <section className="feed-panel">
      <div className="table-header" style={{ gridTemplateColumns }}>
        {table.getHeaderGroups().map((group) =>
          group.headers.map((header) =>
            header.column.getCanSort() ? (
              <button key={header.id} onClick={header.column.getToggleSortingHandler()}>
                {flexRender(header.column.columnDef.header, header.getContext())}
              </button>
            ) : (
              <div key={header.id} aria-hidden="true" />
            ),
          ),
        )}
      </div>
      <div
        className="table-scroll"
        ref={scrollRef}
        onScroll={onScroll}
        role="region"
        aria-label={t('alertHistory')}
        tabIndex={0}
      >
        {rows.length === 0 ? (
          <div className="empty-state">{t('waitingAlerts')}</div>
        ) : (
          <div className="virtual-body" style={{ height: virtualizer.getTotalSize() }}>
            {virtualizer.getVirtualItems().map((item) => {
              const row = rows[item.index]
              return (
                <div
                  key={row.id}
                  className="table-row"
                  style={{
                    transform: `translateY(${item.start}px)`,
                    gridTemplateColumns,
                  }}
                  title={
                    row.original.terminalError ? localizeDiagnostic(row.original.terminalError) : undefined
                  }
                >
                  {row.getVisibleCells().map((cell) => (
                    <div key={cell.id} className="table-cell">
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </div>
                  ))}
                </div>
              )
            })}
          </div>
        )}
        {hasMore && <div className="more-row">{t('olderAlerts')}</div>}
      </div>
      {unseen > 0 && (
        <button
          className="new-alerts"
          onClick={() => {
            setPausedAt(null)
            virtualizer.scrollToOffset(0)
          }}
        >
          <ArrowUp size={12} />
          {t('newAlerts', { count: unseen })}
        </button>
      )}
    </section>
  )
}
