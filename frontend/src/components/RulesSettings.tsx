import { useState } from 'react'
import { Plus } from 'lucide-react'
import type { Rule } from '../api'
import { RuleDialog } from './RuleDialog'
import { AlertBadge } from './AlertBadge'
import { useTranslation, soundLabel } from '../i18n'
import { Button } from './ui'

type Props = { rules: Rule[]; sounds: string[]; onSaved: () => void }

export function RulesSettings({ rules, sounds, onSaved }: Props) {
  const { t, i18n } = useTranslation()
  const [editing, setEditing] = useState(false)
  const [selected, setSelected] = useState<Rule | null>(null)
  return (
    <>
      <div className="rule-list">
        {rules.length === 0 && <div className="rules-empty">{t('noAlerts')}</div>}
        {rules.map((rule) => (
          <button
            type="button"
            className="rule-card"
            key={rule.id}
            onClick={() => {
              setSelected(rule)
              setEditing(true)
            }}
          >
            <AlertBadge name={rule.name} color={rule.color} />
            <span className="rule-meta">
              <strong>
                ±{rule.percent}% /{' '}
                {rule.windowSeconds < 60
                  ? `${rule.windowSeconds}${t('secondsShort')}`
                  : `${rule.windowSeconds / 60}${t('minutesShort')}`}
              </strong>
              <span>
                $
                {new Intl.NumberFormat(i18n.language, {
                  notation: 'compact',
                }).format(rule.minQuoteVolume)}{' '}
                / {t('hours24')}
              </span>
              <span>
                {rule.sound.startsWith('custom:') ? t('customSound') : soundLabel(rule.sound)}
                {rule.sound !== 'none' && ` · ${Math.round(rule.volume * 100)}%`}
              </span>
            </span>
            {rule.terminals.length > 0 && (
              <span className="rule-terminals">
                {rule.terminals.map((id) => (id === 'vataga' ? 'Vataga' : 'Tiger')).join(' · ')} · {t('auto')}
              </span>
            )}
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
          {t('newAlert')}
        </Button>
      </div>
      <RuleDialog
        open={editing}
        onOpenChange={setEditing}
        rule={selected}
        sounds={sounds}
        onSaved={onSaved}
      />
    </>
  )
}
