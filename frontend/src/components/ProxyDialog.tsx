import { useState } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import { Trash2, X } from 'lucide-react'
import type { ProxyProfile, Settings } from '../api'
import { desktop } from '../api'
import { useTranslation } from '../i18n'
import { localizeDiagnostic } from '../i18n/diagnostics'
import { nextProxyName, parseProxy } from '../proxies'
import { Button, Checkbox, Input } from './ui'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  profile: ProxyProfile | null
  settings: Settings
  onSaved: () => void
}

export function ProxyDialog(props: Props) {
  return (
    <Dialog.Root open={props.open} onOpenChange={props.onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="dialog-overlay" />
        <Dialog.Content className="dialog-content">
          <ProxyEditor {...props} />
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

function ProxyEditor({ profile, settings, onSaved, onOpenChange }: Props) {
  const { t } = useTranslation()
  const [draft, setDraft] = useState<ProxyProfile>(
    profile ?? {
      id: '',
      name: nextProxyName(settings.proxies),
      host: '',
      port: 443,
      protocol: 'socks5',
      login: '',
      password: '',
    },
  )
  const [use, setUse] = useState(!profile || profile.id === settings.activeProxyId)
  const [saving, setSaving] = useState(false)
  const [deleteArmed, setDeleteArmed] = useState(false)
  const [error, setError] = useState('')
  async function save(event: React.FormEvent) {
    event.preventDefault()
    setSaving(true)
    setError('')
    try {
      await desktop.saveProxy(
        {
          ...draft,
          protocol: draft.login || draft.password ? 'http' : 'socks5',
        },
        use,
      )
      onSaved()
      onOpenChange(false)
    } catch (cause) {
      setError(String(cause))
    } finally {
      setSaving(false)
    }
  }
  async function remove() {
    if (!profile) return
    if (!deleteArmed) {
      setDeleteArmed(true)
      return
    }
    setSaving(true)
    try {
      await desktop.deleteProxy(profile.id)
      onSaved()
      onOpenChange(false)
    } catch (cause) {
      setError(String(cause))
    } finally {
      setSaving(false)
    }
  }
  return (
    <>
      <div className="dialog-heading">
        <Dialog.Title>{t(profile ? 'editProxy' : 'addProxy')}</Dialog.Title>
        <Dialog.Close asChild>
          <Button size="icon" variant="ghost" aria-label={t('close')}>
            <X size={15} />
          </Button>
        </Dialog.Close>
      </div>
      <Dialog.Description className="sr-only">{t('proxyDescription')}</Dialog.Description>
      <form className="form-stack" onSubmit={save}>
        <label>
          {t('name')}
          <Input
            required
            maxLength={60}
            value={draft.name}
            onChange={(event) => setDraft({ ...draft, name: event.target.value })}
          />
        </label>
        <div className="form-grid proxy-endpoint">
          <label>
            {t('proxyHost')}
            <Input
              required
              value={draft.host}
              placeholder="127.0.0.1"
              onPaste={(event) => {
                const parsed = parseProxy(event.clipboardData.getData('text'))
                if (parsed) {
                  event.preventDefault()
                  setDraft((current) => ({ ...current, ...parsed }))
                }
              }}
              onChange={(event) => setDraft({ ...draft, host: event.target.value })}
              onBlur={() => {
                const parsed = parseProxy(draft.host)
                if (parsed) setDraft((current) => ({ ...current, ...parsed }))
              }}
            />
          </label>
          <label>
            {t('proxyPort')}
            <Input
              required
              type="number"
              min={1}
              max={65535}
              value={draft.port}
              onChange={(event) => setDraft({ ...draft, port: Number(event.target.value) })}
            />
          </label>
        </div>
        <label>
          {t('proxyLogin')}
          <Input
            autoComplete="off"
            value={draft.login}
            onChange={(event) => setDraft({ ...draft, login: event.target.value })}
          />
        </label>
        <label>
          {t('proxyPassword')}
          <Input
            type="password"
            autoComplete="new-password"
            value={draft.password}
            onChange={(event) => setDraft({ ...draft, password: event.target.value })}
          />
        </label>
        <label className="checkbox-row">
          <Checkbox checked={use} onChange={(event) => setUse(event.target.checked)} />
          {t('useProxy')}
        </label>
        {error && <div className="inline-error">{localizeDiagnostic(error)}</div>}
        <div className="dialog-actions">
          {profile && (
            <Button
              type="button"
              variant="ghost"
              className="delete-rule"
              disabled={saving}
              onClick={() => void remove()}
            >
              <Trash2 size={12} />
              {t(deleteArmed ? 'confirmDelete' : 'delete')}
            </Button>
          )}
          <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
            {t('cancel')}
          </Button>
          <Button type="submit" variant="primary" disabled={saving}>
            {t(saving ? 'saving' : 'saveProxy')}
          </Button>
        </div>
      </form>
    </>
  )
}
