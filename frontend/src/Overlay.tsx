import { useEffect, useRef, useState } from 'react'
import { AnimatePresence, motion, useReducedMotion } from 'motion/react'
import { X } from 'lucide-react'
import { ExchangeIcon } from './components/ExchangeIcon'
import { AlertBadge } from './components/AlertBadge'
import type { Alert } from './api'
import { desktop, events } from './api'
import { Ticker } from './components/Ticker'
import { useTranslation } from './i18n'
import { localizeDiagnostic } from './i18n/diagnostics'
import { syncLanguage } from './i18n/language'
import { notificationCards } from './alerts'

export function Overlay() {
  const { t } = useTranslation()
  const [cards, setCards] = useState<Alert[]>([])
  const [error, setError] = useState('')
  const reducedMotion = useReducedMotion()
  const latestID = useRef(0)
  const dismissed = useRef(new Set<number>())

  useEffect(() => {
    function accept(alerts: Alert[]) {
      for (const alert of alerts) latestID.current = Math.max(latestID.current, alert.id)
      const visible = alerts.filter((alert) => !dismissed.current.has(alert.id))
      if (visible.length) setCards((current) => notificationCards(current, visible))
    }
    const stopLanguage = syncLanguage(
      desktop.snapshot().then((snapshot) => snapshot.settings),
      (error) => setError(String(error)),
    )
    const stop = events.alerts(accept)
    void desktop
      .sessionAlerts()
      .then(accept)
      .catch((cause: unknown) => setError(String(cause)))
    const stopRules = events.rules((rules) => {
      const colors = new Map(rules.map((rule) => [rule.id, rule.color]))
      setCards((current) => current.map((card) => ({ ...card, color: colors.get(card.ruleId) ?? card.color })))
    })
    return () => {
      stopLanguage()
      stop()
      stopRules()
    }
  }, [])

  function close(id: number) {
    dismissed.current.add(id)
    setCards((current) => current.filter((card) => card.id !== id))
  }

  async function open(alert: Alert) {
    try {
      const result = await desktop.openTerminals(
        alert.id,
        alert.symbol,
        alert.terminals.length ? alert.terminals : ['vataga'],
      )
      setError(result.error)
      if (result.opened > 0) close(alert.id)
    } catch (cause) {
      setError(`terminal: ${String(cause)}`)
    }
  }

  return (
    <div className="overlay-root">
      {error && (
        <div className="overlay-error" onClick={() => setError('')}>
          {localizeDiagnostic(error)}
        </div>
      )}
      <AnimatePresence
        mode="popLayout"
        onExitComplete={() => {
          if (cards.length === 0) void desktop.hideOverlay(latestID.current)
        }}
      >
        {cards.map((alert) => (
          <motion.article
            className={`toast toast-${alert.direction}`}
            key={alert.id}
            layout="position"
            initial={reducedMotion ? { opacity: 0 } : { x: 28, opacity: 0, filter: 'blur(5px)', scale: 0.98 }}
            animate={{ x: 0, opacity: 1, filter: 'blur(0px)', scale: 1 }}
            exit={reducedMotion ? { opacity: 0 } : { x: 32, opacity: 0, filter: 'blur(4px)', scale: 0.98 }}
            transition={{ duration: reducedMotion ? 0 : 0.18, ease: [0.2, 0.8, 0.2, 1] }}
            onContextMenu={(event) => {
              event.preventDefault()
              close(alert.id)
            }}
          >
            <button
              className="toast-main"
              aria-label={t('openSymbol', { symbol: alert.symbol })}
              onClick={() => void open(alert)}
              title={t('openTerminal')}
            >
              <div className="toast-middle">
                <span className="toast-symbol">
                  <ExchangeIcon />
                  <strong>
                    <Ticker symbol={alert.symbol} />
                  </strong>
                </span>
                <span className="toast-change">
                  {alert.changePercent > 0 ? '+' : ''}
                  {alert.changePercent.toFixed(2)}%
                </span>
              </div>
              <div className="toast-bottom">
                <AlertBadge name={alert.ruleName} color={alert.color} />
                <time>{new Date(alert.createdAtMs).toLocaleTimeString('en-GB', { hour12: false })}</time>
              </div>
            </button>
            <button className="toast-close" onClick={() => close(alert.id)} aria-label={t('dismissAlert')}>
              <X size={14} />
            </button>
          </motion.article>
        ))}
      </AnimatePresence>
    </div>
  )
}
