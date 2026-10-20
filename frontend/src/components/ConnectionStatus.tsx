import type { Health } from '../api'
import { quoteAgeState } from '../connection'
import { useTranslation } from '../i18n'
import { localizeDiagnostic } from '../i18n/diagnostics'
import { desktop } from '../api'
import { Button } from './ui'
import { RotateCw, Wifi } from 'lucide-react'
import { useState } from 'react'

export function ConnectionStatus({ health }: { health: Health | null }) {
  const { t } = useTranslation()
  const [error, setError] = useState('')
  const online = health?.state === 'connected' || health?.state === 'warming' || health?.state === 'degraded'
  const age = online ? health.quoteAgeMs : null
  const connecting = !health || ['starting', 'connecting', 'reconnecting'].includes(health.state)
  const state = online ? quoteAgeState(age) : connecting ? 'warning' : 'offline'

  return (
    <div
      className={`connection-status latency-${state}`}
      role="status"
      title={`${health ? localizeDiagnostic(health.detail) : t('connecting')} · ${t('pingDescription')}: ${health?.pingMs ?? '—'} ms${health?.serviceError ? ` · ${health.serviceError}` : ''}`}
    >
      <Wifi size={14} aria-hidden="true" />
      {online ? (
        <span className="connection-ping">
          {age === null ? '—' : age}<span> {t('ms')}</span>
        </span>
      ) : (
        <span className="connection-label">
          {t(connecting
            ? health?.viaProxy ? 'proxyConnecting' : 'binanceConnecting'
            : health?.viaProxy ? 'proxyUnavailable' : 'binanceDisconnected')}
        </span>
      )}
      {health?.storage.error && (
        <span className="connection-action-error" title={`${health.storage.error} · ${health.storage.unsaved}`}>
          ! {health.storage.unsaved}
        </span>
      )}
      {(!online || health?.state === 'degraded') && (
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
