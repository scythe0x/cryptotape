import { useState } from 'react'
import { Check, Plus } from 'lucide-react'
import type { ProxyProfile, Settings } from '../api'
import { desktop } from '../api'
import { useTranslation } from '../i18n'
import { localizeDiagnostic } from '../i18n/diagnostics'
import { ProxyDialog } from './ProxyDialog'
import { Button } from './ui'

export function ProxySettings({ settings, onSaved }: { settings: Settings; onSaved: () => void }) {
  const { t } = useTranslation()
  const [editing, setEditing] = useState(false)
  const [selected, setSelected] = useState<ProxyProfile | null>(null)
  const [error, setError] = useState('')
  async function direct() {
    try {
      await desktop.selectProxy('')
      setError('')
      onSaved()
    } catch (cause) {
      setError(String(cause))
    }
  }
  return (
    <>
      <div className="rule-list proxy-list">
        <button
          type="button"
          className={`rule-card proxy-card ${!settings.activeProxyId ? 'is-active' : ''}`}
          aria-pressed={!settings.activeProxyId}
          onClick={() => void direct()}
        >
          <strong>{t('noProxy')}</strong>
          {!settings.activeProxyId && <Check size={13} />}
        </button>
        {settings.proxies.map((proxy) => (
          <button
            type="button"
            key={proxy.id}
            className={`rule-card proxy-card ${settings.activeProxyId === proxy.id ? 'is-active' : ''}`}
            onClick={() => {
              setSelected(proxy)
              setEditing(true)
            }}
          >
            <span>
              <strong>{proxy.name}</strong>
              <span className="proxy-address">
                {proxy.host}:{proxy.port}
              </span>
            </span>
            {settings.activeProxyId === proxy.id && <Check size={13} />}
          </button>
        ))}
        <Button
          className="new-rule"
          onClick={() => {
            setSelected(null)
            setEditing(true)
          }}
        >
          <Plus size={13} />
          {t('addProxy')}
        </Button>
      </div>
      {error && <div className="inline-error">{localizeDiagnostic(error)}</div>}
      <ProxyDialog
        open={editing}
        onOpenChange={setEditing}
        profile={selected}
        settings={settings}
        onSaved={onSaved}
      />
    </>
  )
}
