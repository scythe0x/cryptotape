import { useEffect, useState } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import * as Tabs from '@radix-ui/react-tabs'
import { Play, Upload, X } from 'lucide-react'
import type { Monitor, Rule, Settings } from '../api'
import { desktop } from '../api'
import { Button, Input } from './ui'
import { RulesSettings } from './RulesSettings'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  settings: Settings
  rules: Rule[]
  sounds: string[]
  onSaved: () => void
  onImported: (id: string) => void
}

export function SettingsDialog({ open, onOpenChange, settings, rules, sounds, onSaved, onImported }: Props) {
  const [draft, setDraft] = useState(settings)
  const [monitors, setMonitors] = useState<Monitor[]>([])
  const [message, setMessage] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    void desktop.monitors().then(setMonitors).catch((error: unknown) => setMessage(String(error)))
  }, [open])

  async function testProxy() {
    setMessage('testing Binance route…')
    try {
      await desktop.testProxy(draft.proxy)
      setMessage('Binance connection is working')
    } catch (error) {
      setMessage(`connection failed: ${String(error)}`)
    }
  }

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

  async function importSound(file: File | undefined) {
    if (!file) return
    if (file.size > 2 * 1024 * 1024) {
      setMessage('sound must be smaller than 2 MB')
      return
    }
    const extension = file.name.toLowerCase().endsWith('.wav') ? '.wav' : '.mp3'
    try {
      const encoded = await new Promise<string>((resolve, reject) => {
        const reader = new FileReader()
        reader.onload = () => resolve(String(reader.result).split(',')[1] ?? '')
        reader.onerror = () => reject(reader.error)
        reader.readAsDataURL(file)
      })
      const id = await desktop.importSound(file.name.slice(0, -extension.length), extension, encoded)
      onImported(id)
      setMessage('sound imported')
    } catch (error) {
      setMessage(String(error))
    }
  }

  return <Dialog.Root open={open} onOpenChange={onOpenChange}>
    <Dialog.Portal>
      <Dialog.Overlay className="dialog-overlay" />
      <Dialog.Content className="dialog-content settings-dialog">
        <div className="dialog-heading">
          <Dialog.Title>settings</Dialog.Title>
          <Dialog.Close asChild><Button size="icon" variant="ghost" aria-label="close"><X size={15} /></Button></Dialog.Close>
        </div>
        <Dialog.Description className="sr-only">configure market routing and notifications</Dialog.Description>
        <Tabs.Root defaultValue="alerts">
          <Tabs.List className="settings-tabs" aria-label="settings sections">
            <Tabs.Trigger value="alerts">alerts</Tabs.Trigger>
            <Tabs.Trigger value="general">general</Tabs.Trigger>
          </Tabs.List>
          <Tabs.Content value="alerts" className="settings-tab-content">
            <RulesSettings rules={rules} sounds={sounds} onSaved={onSaved} />
          </Tabs.Content>
          <Tabs.Content value="general">
        <form onSubmit={save} className="form-stack">
          <label>http connect proxy
            <div className="input-row">
              <Input value={draft.proxy} placeholder="ip:port (optional)"
                onChange={(event) => setDraft({ ...draft, proxy: event.target.value })} />
              <Button type="button" onClick={() => void testProxy()}>test</Button>
            </div>
          </label>
          <div className="form-grid">
            <label>min 24h turnover, usd<Input type="number" min="0" step="1000000" value={draft.minQuoteVolume}
              onChange={(event) => setDraft({ ...draft, minQuoteVolume: Number(event.target.value) })} /></label>
            <label>rule cooldown, sec<Input type="number" min="0" max="3600" value={draft.cooldownSeconds}
              onChange={(event) => setDraft({ ...draft, cooldownSeconds: Number(event.target.value) })} /></label>
          </div>
          <label>notification monitor
            <select className="input" value={draft.monitorId}
              onChange={(event) => setDraft({ ...draft, monitorId: event.target.value })}>
              <option value="">primary monitor</option>
              {monitors.map((monitor) => <option value={monitor.id} key={monitor.id}>
                {monitor.name || monitor.id}{monitor.primary ? ' · primary' : ''}
              </option>)}
            </select>
          </label>
          <div className="sound-import">
            <strong>sounds</strong>
            <label className="button button-secondary import-button"><Upload size={13} /> import
              <input type="file" accept=".mp3,.wav,audio/mpeg,audio/wav" hidden
                onChange={(event) => void importSound(event.target.files?.[0])} />
            </label>
          </div>
          <div className="sound-chips">
            {sounds.map((sound) => <button type="button" key={sound}
              onClick={() => void desktop.previewSound(sound)} title={`play ${sound}`}>
              <Play size={10} /> {sound.startsWith('custom:') ? sound.slice(7).replace(/^\d+-/, '') : sound}
            </button>)}
          </div>
          {message && <div className={message.includes('failed') ? 'inline-error' : 'muted'}>{message}</div>}
          <div className="dialog-actions">
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>cancel</Button>
            <Button type="submit" variant="primary" disabled={saving}>{saving ? 'saving…' : 'save settings'}</Button>
          </div>
        </form>
          </Tabs.Content>
        </Tabs.Root>
      </Dialog.Content>
    </Dialog.Portal>
  </Dialog.Root>
}
