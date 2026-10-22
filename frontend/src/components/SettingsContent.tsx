import { useEffect, useState } from 'react'
import * as Tabs from '@radix-ui/react-tabs'
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
import { TableColumns } from './TableColumns'

type Props = {
  settings: Settings
  rules: Rule[]
  sounds: string[]
  onSaved: () => void
  onClose: () => void
}

export function SettingsContent({ onClose, settings, rules, sounds, onSaved }: Props) {
  const { t } = useTranslation()
  const [draft, setDraft] = useState(settings)
  const [monitors, setMonitors] = useState<Monitor[]>([])
  const [message, setMessage] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    void desktop
      .monitors()
      .then(setMonitors)
      .catch((error: unknown) => setMessage(String(error)))
  }, [])

  async function save(event: React.FormEvent) {
    event.preventDefault()
    setSaving(true)
    try {
      await desktop.saveSettings({
        ...settings,
        language: draft.language,
        bannersEnabled: draft.bannersEnabled,
        tigerPort: draft.tigerPort,
        tigerLinkGroup: draft.tigerLinkGroup,
        monitorId: draft.monitorId,
      })
      onSaved()
      onClose()
    } catch (error) {
      setMessage(String(error))
    } finally {
      setSaving(false)
    }
  }

  return (
    <main className="settings-page">
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
        <Tabs.Content value="columns" className="settings-tab-content">
          <TableColumns settings={settings} onError={setMessage} />
          {message && <div className="inline-error">{localizeDiagnostic(message)}</div>}
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
              <Button type="button" variant="ghost" onClick={onClose}>
                {t('cancel')}
              </Button>
              <Button type="submit" variant="primary" disabled={saving}>
                {saving ? t('saving') : t('saveSettings')}
              </Button>
            </div>
          </form>
        </Tabs.Content>
      </ScrollArea>
    </main>
  )
}
