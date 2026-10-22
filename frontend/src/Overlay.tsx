import type { CSSProperties } from 'react'
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { AnimatePresence, motion, useReducedMotion } from 'motion/react'
import { X } from 'lucide-react'
import { ExchangeIcon } from './components/ExchangeIcon'
import type { Alert, Settings } from './api'
import { desktop, events } from './api'
import { Ticker } from './components/Ticker'
import { useTranslation, applyLanguage } from './i18n'
import { localizeDiagnostic } from './i18n/diagnostics'
import { notificationCards } from './alerts'
import { useAlertRendered } from './useAlertRendered'
import { BannerTimer } from './components/BannerTimer'
import { exchangeLabel, listingEventLabelKey } from './listings'

export function Overlay() {
  const { t } = useTranslation()
  const [cards, setCards] = useState<Alert[]>([])
  const [error, setError] = useState('')
  const reducedMotion = useReducedMotion()
  const latestID = useRef(0)
  const dismissed = useRef(new Set<number>())
  const stack = useRef<HTMLDivElement>(null)

  useEffect(() => {
    let stopped = false
    let settingsChanged = false
    let bannersEnabled = false
    let bannerVersion = 0
    let stopAlerts = () => {}

    function configure(settings: Settings) {
      applyLanguage(settings.language)
      if (bannersEnabled === settings.bannersEnabled) return
      bannerVersion++
      bannersEnabled = settings.bannersEnabled
      stopAlerts()
      if (bannersEnabled) stopAlerts = events.alerts(accept)
      else setCards([])
    }

    const stopSettings = events.settings((settings) => {
      settingsChanged = true
      configure(settings)
    })

    function accept(alerts: Alert[]) {
      for (const alert of alerts) latestID.current = Math.max(latestID.current, alert.id)
      if (stopped || !bannersEnabled) return
      const visible = alerts.filter((alert) => alert.bannerSeconds !== 0 && !dismissed.current.has(alert.id))
      if (visible.length) setCards((current) => notificationCards(current, visible))
    }

    void desktop.snapshot()
      .then(async (snapshot) => {
        if (stopped) return
        if (!settingsChanged) configure(snapshot.settings)
        if (bannersEnabled) {
          const version = bannerVersion
          const alerts = await desktop.sessionAlerts()
          if (version === bannerVersion) accept(alerts)
        }
      })
      .catch((cause: unknown) => {
        if (!stopped) setError(String(cause))
      })
    const stopRules = events.rules((rules) => {
      const colors = new Map(rules.map((rule) => [rule.id, rule.color]))
      setCards((current) =>
        current.map((card) => ({ ...card, color: colors.get(card.ruleId) ?? card.color })),
      )
    })
    return () => {
      stopped = true
      stopSettings()
      stopAlerts()
      stopRules()
    }
  }, [])

  useLayoutEffect(() => {
    const element = stack.current
    if (!element || cards.length === 0) return
    let lastHeight = 0
    const afterID = latestID.current
    function measure() {
      const height = Math.ceil(element?.offsetHeight ?? 0)
      if (height <= 0 || height === lastHeight) return
      lastHeight = height
      void desktop.resizeOverlay(height, afterID).catch((cause: unknown) => setError(String(cause)))
    }
    const observer = new ResizeObserver(measure)
    observer.observe(element)
    measure()
    return () => {
      observer.disconnect()
    }
  }, [cards])

  useAlertRendered(
    cards.map((card) => card.id),
    'overlay',
  )

  const close = useCallback((id: number) => {
    dismissed.current.add(id)
    setCards((current) => current.filter((card) => card.id !== id))
  }, [])

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
    <div className="overlay-root" ref={stack}>
      {error && (
        <div className="overlay-error" onClick={() => setError('')}>
          {localizeDiagnostic(error)}
        </div>
      )}
      <AnimatePresence
        mode="sync"
        onExitComplete={() => {
          if (cards.length === 0) void desktop.hideOverlay(latestID.current)
        }}
      >
        {cards.map((alert) => (
          <motion.article
            className={`toast toast-${alert.kind === 'listing' ? 'listing' : alert.direction}`}
            style={{ '--alert-color': alert.color } as CSSProperties}
            key={alert.id}
            layout="position"
            initial={
              reducedMotion
                ? { opacity: 0 }
                : { x: 28, opacity: 0, filter: 'blur(5px)', scale: 0.98 }
            }
            animate={{ x: 0, opacity: 1, filter: 'blur(0px)', scale: 1 }}
            exit={
              reducedMotion
                ? { opacity: 0 }
                : { x: 32, opacity: 0, filter: 'blur(4px)', scale: 0.98 }
            }
            transition={{ duration: reducedMotion ? 0 : 0.18, ease: [0.2, 0.8, 0.2, 1] }}
            onContextMenu={(event) => {
              event.preventDefault()
              close(alert.id)
            }}
          >
            <BannerTimer id={alert.id} seconds={alert.bannerSeconds} onClose={close} />
            <button
              className="toast-main"
              aria-label={t('openSymbol', { symbol: alert.symbol })}
              onClick={() => void open(alert)}
              title={t('openTerminal')}
            >
              <div className="toast-middle">
                <span className="toast-symbol">
                  <ExchangeIcon exchange={alert.listing?.exchange ?? 'binance'} />
                  <strong>
                    <Ticker symbol={alert.symbol} />
                    {alert.kind === 'listing' && alert.listing && (
                      <span className="toast-event"> — {t(listingEventLabelKey(alert.listing.marketType))} {exchangeLabel(alert.listing.exchange)}</span>
                    )}
                  </strong>
                </span>
                {alert.kind !== 'listing' && (
                  <span className="toast-change">
                    {alert.changePercent > 0 ? '+' : ''}
                    {alert.changePercent.toFixed(2)}%
                  </span>
                )}
              </div>
            </button>
            <button
              className="toast-close"
              onClick={() => close(alert.id)}
              aria-label={t('dismissAlert')}
            >
              <X size={14} />
            </button>
          </motion.article>
        ))}
      </AnimatePresence>
    </div>
  )
}
