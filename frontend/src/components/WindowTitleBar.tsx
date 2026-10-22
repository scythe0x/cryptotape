import { useEffect, useState, type ReactNode } from 'react'
import { Window } from '@wailsio/runtime'
import { Copy, Minus, Square, X } from 'lucide-react'
import { useTranslation } from '../i18n'

export function WindowTitleBar({ title, status, actions, navigation, onClose, onError }: {
  navigation?: ReactNode
  title?: string
  status?: ReactNode
  actions?: ReactNode
  onClose?: () => void
  onError: (message: string) => void
}) {
  const { t } = useTranslation()
  const [maximised, setMaximised] = useState(false)

  useEffect(() => {
    let active = true
    function update() {
      void Window.IsMaximised().then((value) => {
        if (active) setMaximised(value)
      }).catch((cause: unknown) => {
        if (active) onError(String(cause))
      })
    }
    update()
    window.addEventListener('resize', update)
    return () => {
      active = false
      window.removeEventListener('resize', update)
    }
  }, [onError])

  function run(action: () => Promise<void>) {
    void action().catch((cause: unknown) => onError(String(cause)))
  }

  const maximiseLabel = t(maximised ? 'restoreWindow' : 'maximise')
  return (
    <header className="window-titlebar" data-window-toolbar>
      {navigation ?? <>
        <span className="window-brand">Mythra</span>
        {title && <span className="window-title">{title}</span>}
        {status}
      </>}
      <div className="connection-actions">
        {actions}
        <div className="window-controls">
          <button type="button" aria-label={t('minimise')} title={t('minimise')} onClick={() => run(() => Window.Minimise())}>
            <Minus size={14} aria-hidden="true" />
          </button>
          <button type="button" aria-label={maximiseLabel} title={maximiseLabel} onClick={() => run(() => Window.ToggleMaximise())}>
            {maximised ? <Copy size={12} aria-hidden="true" /> : <Square size={12} aria-hidden="true" />}
          </button>
          <button type="button" className="window-close" aria-label={t('close')} title={t('close')} onClick={() => onClose ? onClose() : run(() => Window.Close())}>
            <X size={16} aria-hidden="true" />
          </button>
        </div>
      </div>
    </header>
  )
}
