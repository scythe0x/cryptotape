import { useRef, useState, type RefObject } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import { Trash2, X } from 'lucide-react'
import type { Rule, TerminalID } from '../api'
import { desktop } from '../api'
import { Button, Input } from './ui'
import { ColorPicker } from './ColorPicker'
import { SoundField } from './SoundField'
import { useTranslation } from '../i18n'
import { localizeDiagnostic } from '../i18n/diagnostics'
import { TerminalButton, terminals } from './TerminalButton'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  rule: Rule | null
  sounds: string[]
  onSaved: () => void
}
const emptyRule: Rule = {
  id: 0,
  name: '',
  percent: 5,
  windowSeconds: 60,
  sound: 'chime',
  color: '#a78bfa',
  volume: 1,
  minQuoteVolume: 80_000_000,
  terminals: [],
}

export function RuleDialog(props: Props) {
  const nameField = useRef<HTMLInputElement>(null)
  return (
    <Dialog.Root open={props.open} onOpenChange={props.onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="dialog-overlay" />
        <Dialog.Content
          className="dialog-content"
          onOpenAutoFocus={(event) => {
            event.preventDefault()
            nameField.current?.focus()
          }}
        >
          <RuleEditor {...props} nameField={nameField} />
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

function RuleEditor({
  onOpenChange,
  rule,
  sounds,
  onSaved,
  nameField,
}: Props & { nameField: RefObject<HTMLInputElement | null> }) {
  const { t } = useTranslation()
  const [draft, setDraft] = useState<Rule>(rule ?? emptyRule)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [deleteArmed, setDeleteArmed] = useState(false)

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

  async function remove() {
    if (!rule) return
    if (!deleteArmed) {
      setDeleteArmed(true)
      return
    }
    setSaving(true)
    try {
      await desktop.deleteRule(rule.id)
      onSaved()
      onOpenChange(false)
    } catch (cause) {
      setError(String(cause))
    } finally {
      setSaving(false)
    }
  }

  function toggleTerminal(id: TerminalID) {
    setDraft((current) => ({
      ...current,
      terminals: current.terminals.includes(id)
        ? current.terminals.filter((value) => value !== id)
        : [...current.terminals, id],
    }))
  }

  return (
    <>
      <div className="dialog-heading">
        <Dialog.Title>{rule ? t('editAlert') : t('newAlert')}</Dialog.Title>
        <Dialog.Close asChild>
          <Button size="icon" variant="ghost" aria-label={t('close')}>
            <X size={15} />
          </Button>
        </Dialog.Close>
      </div>
      <Dialog.Description className="sr-only">{t('alertDescription')}</Dialog.Description>
      <form onSubmit={save} className="form-stack">
        <div className="name-field">
          <label htmlFor="rule-name">{t('name')}</label>
          <div className="input-row">
            <Input
              id="rule-name"
              ref={nameField}
              required
              maxLength={60}
              value={draft.name}
              onChange={(event) => setDraft({ ...draft, name: event.target.value })}
              placeholder={t('namePlaceholder')}
            />
            <ColorPicker value={draft.color} onChange={(color) => setDraft({ ...draft, color })} />
          </div>
        </div>
        <div className="form-grid">
          <label>
            {t('movement')}
            <Input
              type="number"
              required
              min="0.1"
              max="100"
              step="0.1"
              value={draft.percent}
              onChange={(event) => setDraft({ ...draft, percent: Number(event.target.value) })}
            />
          </label>
          <label>
            {t('window')}
            <Input
              type="number"
              required
              min="5"
              max="600"
              step="1"
              value={draft.windowSeconds}
              onChange={(event) => setDraft({ ...draft, windowSeconds: Number(event.target.value) })}
            />
          </label>
        </div>
        <label>
          {t('minTurnover')}
          <Input
            type="number"
            required
            min="0"
            step="1000000"
            value={draft.minQuoteVolume}
            onChange={(event) => setDraft({ ...draft, minQuoteVolume: Number(event.target.value) })}
          />
        </label>
        <SoundField
          sound={draft.sound}
          volume={draft.volume}
          sounds={sounds}
          onChange={(sound) => setDraft((current) => ({ ...current, sound }))}
          onVolume={(volume) => setDraft((current) => ({ ...current, volume }))}
          onError={setError}
        />
        <fieldset className="terminal-field">
          <legend>{t('openAutomatically')}</legend>
          <div className="terminal-choices">
            {terminals.map(({ id }) => (
              <TerminalButton
                key={id}
                id={id}
                toggle
                active={draft.terminals.includes(id)}
                onClick={() => toggleTerminal(id)}
              />
            ))}
          </div>
        </fieldset>
        {error && <div className="inline-error">{localizeDiagnostic(error)}</div>}
        <div className="dialog-actions">
          {rule && (
            <Button
              type="button"
              className="delete-rule"
              variant="ghost"
              disabled={saving}
              onClick={() => void remove()}
            >
              <Trash2 size={12} />
              {deleteArmed ? t('confirmDelete') : t('delete')}
            </Button>
          )}
          <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
            {t('cancel')}
          </Button>
          <Button type="submit" variant="primary" disabled={saving}>
            {saving ? t('saving') : t('saveAlert')}
          </Button>
        </div>
      </form>
    </>
  )
}
