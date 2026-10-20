import { useEffect, useState } from 'react'
import { AnimatePresence, motion } from 'motion/react'
import { ArrowDownRight, ArrowUpRight, X } from 'lucide-react'
import { siBinance } from 'simple-icons'
import type { Alert } from './api'
import { desktop, events } from './api'

export function Overlay() {
  const [cards, setCards] = useState<Alert[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    const stop = events.alert((alert) => {
      setCards((current) => [...current.filter((card) => card.id !== alert.id), alert].slice(-3))
    })
    void desktop.sessionAlerts().then((alerts) => {
      setCards((current) => [...new Map([...alerts, ...current].map((alert) => [alert.id, alert])).values()].slice(-3))
    }).catch((cause: unknown) => setError(String(cause)))
    return stop
  }, [])

  function close(id: number) {
    setCards((current) => current.filter((card) => card.id !== id))
  }

  async function open(alert: Alert) {
    try {
      await desktop.openTerminal(alert.id, alert.symbol)
      setError('')
    } catch (cause) {
      setError(`terminal: ${String(cause)}`)
    }
  }

  return <div className="overlay-root">
    {error && <div className="overlay-error" onClick={() => setError('')}>{error}</div>}
    <AnimatePresence onExitComplete={() => { if (cards.length === 0) void desktop.hideOverlay() }}>
      {cards.map((alert) => <motion.article className={`toast toast-${alert.direction}`} key={alert.id}
        initial={{ x: 44, opacity: 0, filter: 'blur(9px)', scale: .96 }}
        animate={{ x: 0, opacity: 1, filter: 'blur(0px)', scale: 1 }}
        exit={{ x: 55, opacity: 0, filter: 'blur(7px)', scale: .96 }}
        transition={{ duration: .22, ease: [0.2, 0.8, 0.2, 1] }}
        onContextMenu={(event) => { event.preventDefault(); close(alert.id) }}>
        <button className="toast-main" onClick={() => void open(alert)} title="open in terminal">
          <div className="toast-middle"><span className="toast-symbol">
            <svg viewBox="0 0 24 24" aria-label="Binance"><path d={siBinance.path} /></svg>
            <strong>{alert.symbol}</strong>
          </span><span className="toast-change">
            {alert.direction === 'down' ? <ArrowDownRight size={18} /> : <ArrowUpRight size={18} />}
            {alert.changePercent > 0 ? '+' : ''}{alert.changePercent.toFixed(2)}%
          </span></div>
          <div className="toast-bottom"><span className="alert-badge" title={alert.ruleName}>{alert.ruleName}</span><time>
            {new Date(alert.createdAtMs).toLocaleTimeString('en-GB', { hour12: false })}
          </time></div>
        </button>
        <button className="toast-close" onClick={() => close(alert.id)} aria-label="dismiss alert"><X size={14} /></button>
      </motion.article>)}
    </AnimatePresence>
  </div>
}
