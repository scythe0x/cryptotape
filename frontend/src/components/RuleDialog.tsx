import { useState } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import { Play, X } from 'lucide-react'
import type { Rule } from '../api'
import { desktop } from '../api'
import { Button, Input } from './ui'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  rule: Rule | null
  sounds: string[]
  onSaved: () => void
}

const emptyRule: Rule = {
  id: 0, name: '', percent: 5, windowSeconds: 60,
  sound: 'chime', openTerminal: false, enabled: true,
}

function soundLabel(id: string): string {
  return id.startsWith('custom:') ? id.slice(7).replace(/^\d+-/, '') : id
}

export function RuleDialog({ open, onOpenChange, rule, sounds, onSaved }: Props) {
  const [draft, setDraft] = useState<Rule>(rule ?? emptyRule)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  async function save(event: React.FormEvent) {
    event.preventDefault()
    setSaving(true)
    setError('')
    try {
      await desktop.saveRule(draft)
      onSaved()
      onOpenChange(false)
    } catch (cause) {
      setError(String(cause))
    } finally {
      setSaving(false)
    }
  }

  return <Dialog.Root open={open} onOpenChange={onOpenChange}>
    <Dialog.Portal>
      <Dialog.Overlay className="dialog-overlay" />
      <Dialog.Content className="dialog-content">
        <div className="dialog-heading">
          <Dialog.Title>{rule ? 'edit alert' : 'new alert'}</Dialog.Title>
          <Dialog.Close asChild><Button size="icon" variant="ghost" aria-label="close"><X size={15} /></Button></Dialog.Close>
        </div>
        <Dialog.Description className="sr-only">configure a price movement alert</Dialog.Description>
        <form onSubmit={save} className="form-stack">
          <label>name<Input autoFocus required maxLength={60} value={draft.name}
            onChange={(event) => setDraft({ ...draft, name: event.target.value })} placeholder="e.g. fast knife" /></label>
          <div className="form-grid">
            <label>movement, %<Input type="number" required min="0.1" max="100" step="0.1" value={draft.percent}
              onChange={(event) => setDraft({ ...draft, percent: Number(event.target.value) })} /></label>
            <label>window, seconds<Input type="number" required min="5" max="600" step="1" value={draft.windowSeconds}
              onChange={(event) => setDraft({ ...draft, windowSeconds: Number(event.target.value) })} /></label>
          </div>
          <label>sound
            <div className="input-row">
              <select className="input" value={draft.sound}
                onChange={(event) => setDraft({ ...draft, sound: event.target.value })}>
                {sounds.map((sound) => <option key={sound} value={sound}>{soundLabel(sound)}</option>)}
              </select>
              <Button type="button" size="icon" title="preview sound" onClick={() => void desktop.previewSound(draft.sound)}>
                <Play size={13} />
              </Button>
            </div>
          </label>
          <label className="check-row">
            <input type="checkbox" checked={draft.openTerminal}
              onChange={(event) => setDraft({ ...draft, openTerminal: event.target.checked })} />
            open ticker in terminal automatically
          </label>
          {error && <div className="inline-error">{error}</div>}
          <div className="dialog-actions">
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>cancel</Button>
            <Button type="submit" variant="primary" disabled={saving}>{saving ? 'saving…' : 'save alert'}</Button>
          </div>
        </form>
      </Dialog.Content>
    </Dialog.Portal>
  </Dialog.Root>
}
