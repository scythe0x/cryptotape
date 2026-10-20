import type { Health } from '../api'
import { latencyState } from '../connection'
import { useTranslation } from '../i18n'
import { localizeDiagnostic } from '../i18n/diagnostics'
import { desktop } from '../api'
import { Button } from './ui'
import { RotateCw } from 'lucide-react'
import { useState } from 'react'

export function ConnectionStatus({ health }: { health: Health | null }) {
  const { t } = useTranslation()
  const [error, setError] = useState('')
  const online = health?.state === 'connected' || health?.state === 'warming'
  const ping = online ? health.pingMs : null
  const connecting = !health || ['starting', 'connecting', 'reconnecting'].includes(health.state)
  const state = online ? latencyState(ping) : connecting ? 'warning' : 'offline'

  return (
    <div
      className={`connection-status latency-${state}`}
      role="status"
      title={`${health ? localizeDiagnostic(health.detail) : t('connecting')} · ${t('pingDescription')}`}
    >
      <span className={`connection-dot ${online ? 'is-online' : ''}`} aria-hidden="true" />
      <span className="connection-label">
        {t(
          online
            ? 'connected'
            : connecting
              ? health?.viaProxy
                ? 'proxyConnecting'
                : 'binanceConnecting'
              : health?.viaProxy
                ? 'proxyUnavailable'
                : 'binanceDisconnected',
        )}
      </span>
      {online && (
        <span className="connection-ping">
          {ping === null ? '—' : ping}
          <span> {t('ms')}</span>
        </span>
      )}
      {!online && (
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
