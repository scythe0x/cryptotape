import type { Health } from '../api'
import { latencyState } from '../connection'
import { useTranslation } from '../i18n'
import { localizeDiagnostic } from '../i18n/diagnostics'

export function ConnectionStatus({ health }: { health: Health | null }) {
  const { t } = useTranslation()
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
        {t(online ? 'connected' : connecting ? 'binanceConnecting' : 'binanceDisconnected')}
      </span>
      {online && (
        <span className="connection-ping">
          {ping === null ? '—' : ping}
          <span> {t('ms')}</span>
        </span>
      )}
    </div>
  )
}
