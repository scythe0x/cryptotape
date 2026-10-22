import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import { ArrowUp, Check } from 'lucide-react'
import type { Alert, Rule, TableColumnID, TerminalID } from '../api'
import { desktop, defaultTableColumns } from '../api'
import { minimumTableRowHeight, tableGridTemplate, tableMinimumWidth } from '../table-columns'
import { useTranslation } from '../i18n'
import { localizeDiagnostic } from '../i18n/diagnostics'
import { Ticker } from './Ticker'
import { useAlertRendered } from '../useAlertRendered'
import { AlertBadge } from './AlertBadge'
import { ExchangeIcon } from './ExchangeIcon'
import { TerminalButton, terminals } from './TerminalButton'
import { ListingSummary } from './ListingSummary'
import { ScrollArea } from './ScrollArea'
import './alerts-table.css'

type Props = {
  alerts: Alert[]
  rules: Rule[]
  hasMore: boolean
  loadMore: () => void
  onError: (error: string) => void
  columns?: TableColumnID[]
}

export function sortAlerts(alerts: readonly Alert[]): Alert[] {
  return [...alerts].sort((left, right) => right.createdAtMs - left.createdAtMs || right.id - left.id)
}

export function AlertsTable({ alerts, rules, hasMore, loadMore, onError, columns = defaultTableColumns }: Props) {
  const { t } = useTranslation()
  const [pausedAt, setPausedAt] = useState<number | null>(null)
  const scrollRef = useRef<HTMLDivElement>(null)
  const [copied, setCopied] = useState('')
  useEffect(() => {
    if (!copied) return
    const timer = window.setTimeout(() => setCopied(''), 1200)
    return () => window.clearTimeout(timer)
  }, [copied])

  const copy = useCallback(async (symbol: string) => {
    try {
      await desktop.copySymbol(symbol)
      setCopied(symbol)
    } catch (error) {
      onError(String(error))
    }
  }, [onError])
  const open = useCallback(async (alert: Alert, terminal: TerminalID) => {
    try {
      await desktop.openTerminal(alert.id, alert.symbol, terminal)
    } catch (error) {
      onError(`terminal: ${String(error)}`)
    }
  }, [onError])
  const colors = useMemo(() => new Map(rules.map((rule) => [rule.id, rule.color])), [rules])
  const rows = useMemo(() => sortAlerts(alerts), [alerts])
  const getItemKey = useCallback((index: number) => rows[index].id, [rows])
  const virtualizer = useVirtualizer({
    count: columns.length ? rows.length : 0,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => minimumTableRowHeight,
    overscan: 8,
    getItemKey,
    anchorTo: pausedAt === null ? 'start' : 'end',
  })
  useAlertRendered(virtualizer.getVirtualItems().map((item) => rows[item.index].id), 'feed')
  const unseen = pausedAt === null ? 0 : alerts.filter((alert) => alert.id > pausedAt).length
  const gridTemplateColumns = tableGridTemplate(columns)
  const minimumWidth = tableMinimumWidth(columns)
  const headers: Record<TableColumnID, string> = {
    time: t('time'),
    ticker: t('ticker'),
    event: t('alertEvent'),
    terminals: t('terminal'),
  }

  function onScroll() {
    const element = scrollRef.current
    if (!element) return
    if (element.scrollTop <= 8) {
      setPausedAt(null)
    } else {
      setPausedAt((current) => current ?? alerts.reduce((newest, alert) => Math.max(newest, alert.id), 0))
    }

    if (hasMore && element.scrollHeight - element.scrollTop - element.clientHeight < 260) {
      loadMore()
    }
  }

  function renderCell(alert: Alert, column: TableColumnID) {
    switch (column) {
      case 'time':
        return new Date(alert.createdAtMs).toLocaleTimeString('en-GB', { hour12: false })
      case 'ticker':
        return (
          <button
            className="ticker-button"
            title={t('copySymbol', { symbol: alert.symbol })}
            onClick={() => void copy(alert.symbol)}
          >
            {copied === alert.symbol ? <Check size={12} /> : <ExchangeIcon exchange={alert.listing?.exchange ?? 'binance'} />}
            <strong><Ticker symbol={alert.symbol} /></strong>
          </button>
        )
      case 'event':
        return (
          <div className="feed-event">
            <AlertBadge name={alert.ruleName} color={colors.get(alert.ruleId) ?? alert.color} />
            {alert.kind === 'listing' ? <ListingSummary alert={alert} /> : (
              <span className={alert.direction === 'down' ? 'negative' : 'positive'}>
                {alert.changePercent > 0 ? '+' : ''}{alert.changePercent.toFixed(2)}%
              </span>
            )}
          </div>
        )
      case 'terminals':
        return (
          <div className="feed-terminals">
            {terminals.map(({ id }) => (
              <TerminalButton key={id} id={id} onClick={() => void open(alert, id)} />
            ))}
          </div>
        )
    }
  }

  return (
    <section
      className="feed-panel measured-feed"
      style={{ minWidth: minimumWidth, '--table-row-min-height': `${minimumTableRowHeight}px` } as CSSProperties}
    >
      {columns.length > 0 && (
        <div className="table-header" data-window-table-header style={{ gridTemplateColumns }}>
          {columns.map((column) => (
            <div key={column} data-column-id={column} className="feed-column-heading">{headers[column]}</div>
          ))}
        </div>
      )}
      <ScrollArea
        className="table-scroll-area"
        viewportClassName="table-scroll"
        viewportRef={scrollRef}
        onScroll={onScroll}
        viewportLabel={t('alertHistory')}
      >
        {rows.length === 0 ? <div className="empty-state">{t('waitingAlerts')}</div> : columns.length > 0 && (
          <div className="virtual-body" style={{ height: virtualizer.getTotalSize() }}>
            {virtualizer.getVirtualItems().map((item) => {
              const alert = rows[item.index]
              return (
                <div
                  key={alert.id}
                  data-index={item.index}
                  ref={virtualizer.measureElement}
                  className="table-row"
                  style={{ transform: `translateY(${item.start}px)`, gridTemplateColumns }}
                  title={alert.terminalError ? localizeDiagnostic(alert.terminalError) : undefined}
                >
                  {columns.map((column) => (
                    <div key={column} className={`table-cell feed-cell-${column}`}>
                      {renderCell(alert, column)}
                    </div>
                  ))}
                </div>
              )
            })}
          </div>
        )}
        {hasMore && columns.length > 0 && <div className="more-row">{t('olderAlerts')}</div>}
      </ScrollArea>
      {unseen > 0 && (
        <button
          className="new-alerts"
          onClick={() => {
            setPausedAt(null)
            virtualizer.scrollToOffset(0)
          }}
        >
          <ArrowUp size={12} />{t('newAlerts', { count: unseen })}
        </button>
      )}
    </section>
  )
}
