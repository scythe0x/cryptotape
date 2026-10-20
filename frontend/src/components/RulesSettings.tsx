import { useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import type { Rule } from '../api'
import { desktop } from '../api'
import { RuleDialog } from './RuleDialog'
import { Button } from './ui'

type Props = {
  rules: Rule[]
  sounds: string[]
  onSaved: () => void
}

function duration(seconds: number): string {
  return seconds < 60 ? `${seconds}s` : `${seconds / 60}m`
}

export function RulesSettings({ rules, sounds, onSaved }: Props) {
  const [editing, setEditing] = useState(false)
  const [selected, setSelected] = useState<Rule | null>(null)
  const [pending, setPending] = useState<number | null>(null)
  const [error, setError] = useState('')

  async function toggle(rule: Rule) {
    setPending(rule.id)
    setError('')
    try {
      await desktop.saveRule({ ...rule, enabled: !rule.enabled })
      onSaved()
    } catch (cause) {
      setError(String(cause))
    } finally {
      setPending(null)
    }
  }

  async function remove(rule: Rule) {
    if (!window.confirm(`delete ${rule.name}?`)) return
    setError('')
    try {
      await desktop.deleteRule(rule.id)
      onSaved()
    } catch (cause) {
      setError(String(cause))
    }
  }

  return <>
    <div className="rules-toolbar">
      <Button onClick={() => { setSelected(null); setEditing(true) }}><Plus size={13} />new alert</Button>
    </div>
    <div className="rule-list">
      {rules.length === 0 && <div className="rules-empty">no alerts configured</div>}
      {rules.map((rule) => <article className={`rule-card ${rule.enabled ? '' : 'rule-disabled'}`} key={rule.id}>
        <div className="rule-card-top">
          <button className="rule-title" onClick={() => { setSelected(rule); setEditing(true) }}>{rule.name}</button>
          <label className="switch" title={rule.enabled ? 'disable alert' : 'enable alert'}>
            <input type="checkbox" aria-label={`enable ${rule.name}`} checked={rule.enabled} disabled={pending !== null}
              onChange={() => void toggle(rule)} /><span />
          </label>
        </div>
        <div className="rule-meta"><strong>±{rule.percent}% / {duration(rule.windowSeconds)}</strong>
          <span>{rule.sound.startsWith('custom:') ? 'custom' : rule.sound}{rule.openTerminal ? ' · auto' : ''}</span>
          <button aria-label={`delete ${rule.name}`} title="delete alert" onClick={() => void remove(rule)}><Trash2 size={12} /></button>
        </div>
      </article>)}
    </div>
    {error && <div className="inline-error">{error}</div>}
    {editing && <RuleDialog open={editing} onOpenChange={setEditing} rule={selected} sounds={sounds} onSaved={onSaved} />}
  </>
}
