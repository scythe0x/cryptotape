import type { Health } from '../api'
import { latencyState } from '../connection'
import { useTranslation } from '../i18n'
import { localizeDiagnostic } from '../i18n/diagnostics'
import { desktop } from '../api'
import { Button } from './ui'
import { LoaderCircle, RotateCw } from 'lucide-react'
import { useState } from 'react'

export function ConnectionStatus({ health }: { health: Health | null }) {
  const { t } = useTranslation()
  const [error, setError] = useState('')
  const online = health?.state === 'connected' || health?.state === 'warming' || health?.state === 'degraded'
  const ping = online ? health.pingMs : null
  const connecting = !health || ['starting', 'connecting', 'reconnecting'].includes(health.state)
  const state = online ? latencyState(ping) : connecting ? 'warning' : 'offline'
  const description = online
    ? localizeDiagnostic(health.detail)
    : connecting
      ? t(health?.viaProxy ? 'proxyConnecting' : 'binanceConnecting')
      : t(health?.viaProxy ? 'proxyUnavailable' : 'binanceDisconnected')

  return (
    <div
      className={`connection-status latency-${state}`}
      role="status"
      title={`${description} · ${t('pingDescription')}: ${health?.pingMs ?? '—'} ms · ${t('quoteAge')}: ${health?.quoteAgeMs ?? '—'} ms${health?.serviceError ? ` · ${health.serviceError}` : ''}`}
    >
      <span className="connection-ping">
        {connecting || (online && ping === null)
          ? <LoaderCircle className="connection-spinner" size={12} aria-label={t('connecting')} />
          : online ? ping : '—'}
        <span> {t('ms')}</span>
      </span>
      {health?.storage.error && (
        <span className="connection-action-error" title={`${health.storage.error} · ${health.storage.unsaved}`}>
          ! {health.storage.unsaved}
        </span>
      )}
      {((!online && !connecting) || health?.state === 'degraded') && (
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label={t('reconnect')}
          title={t('reconnect')}
          onClick={() => {
            setError('')
            void desktop.reconnect().catch((cause: unknown) => setError(String(cause)))
          }}
        >
          <RotateCw size={12} />
        </Button>
      )}
      {error && <span className="connection-action-error">{localizeDiagnostic(error)}</span>}
    </div>
  )
}
