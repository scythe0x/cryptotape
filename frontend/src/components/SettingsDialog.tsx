import { useEffect, useState } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import * as Tabs from '@radix-ui/react-tabs'
import { X } from 'lucide-react'
import type { Monitor, Rule, Settings } from '../api'
import { desktop, languageSchema } from '../api'
import { Button, Checkbox, Input } from './ui'
import { useTranslation } from '../i18n'
import { localizeDiagnostic } from '../i18n/diagnostics'
import { RulesSettings } from './RulesSettings'
import { ProxySettings } from './ProxySettings'
import { ListingSourceSettings } from './ListingSourceSettings'
import { Select } from './Select'
import { ScrollArea } from './ScrollArea'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  settings: Settings
  rules: Rule[]
  sounds: string[]
  onSaved: () => void
}

export function SettingsDialog(props: Props) {
  return (
    <Dialog.Root open={props.open} onOpenChange={props.onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="dialog-overlay" />
        <Dialog.Content className="dialog-content settings-dialog scroll-dialog">
          <SettingsContent {...props} />
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

function SettingsContent({ open, onOpenChange, settings, rules, sounds, onSaved }: Props) {
  const { t } = useTranslation()
  const [draft, setDraft] = useState(settings)
  const [monitors, setMonitors] = useState<Monitor[]>([])
  const [message, setMessage] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    void desktop
      .monitors()
      .then(setMonitors)
      .catch((error: unknown) => setMessage(String(error)))
  }, [open])

  async function save(event: React.FormEvent) {
    event.preventDefault()
    setSaving(true)
    try {
      await desktop.saveSettings(draft)
      onSaved()
      onOpenChange(false)
    } catch (error) {
      setMessage(String(error))
    } finally {
      setSaving(false)
    }
  }

  return (
    <>
      <div className="dialog-heading">
        <Dialog.Title>{t('settings')}</Dialog.Title>
        <Dialog.Close asChild>
          <Button size="icon" variant="ghost" aria-label={t('close')}>
            <X size={15} />
          </Button>
        </Dialog.Close>
      </div>
      <Dialog.Description className="sr-only">{t('settingsDescription')}</Dialog.Description>
      <Tabs.Root defaultValue="alerts">
        <Tabs.List className="settings-tabs" aria-label={t('settingsSections')}>
          <Tabs.Trigger value="alerts">{t('alerts')}</Tabs.Trigger>
          <Tabs.Trigger value="proxies">{t('proxies')}</Tabs.Trigger>
          <Tabs.Trigger value="listings">{t('listingsSource')}</Tabs.Trigger>
          <Tabs.Trigger value="general">{t('general')}</Tabs.Trigger>
        </Tabs.List>
        <ScrollArea className="settings-body-scroll">
          <Tabs.Content value="alerts" className="settings-tab-content">
            <RulesSettings rules={rules} sounds={sounds} onSaved={onSaved} />
          </Tabs.Content>
          <Tabs.Content value="proxies">
            <ProxySettings settings={settings} onSaved={onSaved} />
          </Tabs.Content>
          <Tabs.Content value="listings" className="settings-tab-content">
            <ListingSourceSettings />
          </Tabs.Content>
          <Tabs.Content value="general">
            <form onSubmit={save} className="form-stack">
              <label>
                {t('language')}
                <Select
                  aria-label={t('language')}
                  value={draft.language}
                  onValueChange={(value) => setDraft({ ...draft, language: languageSchema.parse(value) })}
                  options={[{ value: 'en', label: 'English' }, { value: 'ru', label: 'Русский' }]}
                />
              </label>
              <label className="checkbox-row">
                <Checkbox
                  checked={draft.bannersEnabled}
                  onChange={(event) => setDraft({ ...draft, bannersEnabled: event.target.checked })}
                />
                {t('bannersEnabled')}
              </label>
              <div className="form-grid">
                <label>
                  {t('tigerPort')}
                  <Input
                    type="number"
                    required
                    min="1"
                    max="65535"
                    value={draft.tigerPort}
                    onChange={(event) => setDraft({ ...draft, tigerPort: Number(event.target.value) })}
                  />
                </label>
                <label>
                  {t('tigerGroup')}
                  <Select
                    aria-label={t('tigerGroup')}
                    value={draft.tigerLinkGroup}
                    onValueChange={(value) => setDraft({ ...draft, tigerLinkGroup: value })}
                    options={[
                      ...Array.from({ length: 26 }, (_, index) => String.fromCharCode(65 + index)),
                      ...Array.from({ length: 19 }, (_, index) => String(index)),
                    ].map((group) => ({ value: group, label: group }))}
                  />
                </label>
              </div>
              <label>
                {t('monitor')}
                <Select
                  aria-label={t('monitor')}
                  value={draft.monitorId}
                  onValueChange={(value) => setDraft({ ...draft, monitorId: value })}
                  options={[
                    { value: '', label: t('primaryMonitor') },
                    ...monitors.map((monitor) => ({
                      value: monitor.id,
                      label: `${monitor.name || monitor.id}${monitor.primary ? ` · ${t('primary')}` : ''}`,
                    })),
                  ]}
                />
              </label>
              {message && (
                <div className={message.includes('failed') ? 'inline-error' : 'muted'}>
                  {localizeDiagnostic(message)}
                </div>
              )}
              <div className="dialog-actions">
                <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
                  {t('cancel')}
                </Button>
                <Button type="submit" variant="primary" disabled={saving}>
                  {saving ? t('saving') : t('saveSettings')}
                </Button>
              </div>
            </form>
          </Tabs.Content>
        </ScrollArea>
      </Tabs.Root>
    </>
  )
}
