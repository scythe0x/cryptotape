import { useState } from 'react'
import { Plus } from 'lucide-react'
import type { Rule } from '../api'
import { RuleDialog } from './RuleDialog'
import { AlertBadge } from './AlertBadge'
import { useTranslation, soundLabel } from '../i18n'
import { Button } from './ui'
import { exchangeLabel } from '../listings'
import { terminals } from './TerminalButton'
import './controls.css'

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
            <span className="rule-meta-row">
              {rule.terminals.length > 0 && (
                <span className="rule-terminals">
                  {terminals.filter((terminal) => rule.terminals.includes(terminal.id)).map((terminal) => (
                    <img key={terminal.id} className="rule-terminal-icon" src={terminal.icon} alt={terminal.name} title={terminal.name} draggable={false} />
                  ))}
                </span>
              )}
              <span className="rule-meta">
                <strong>
                  {rule.kind === 'listing' ? (
                    t('externalListingAlert')
                  ) : (
                    <>
                      ±{rule.percent}% /{' '}
                      {rule.windowSeconds < 60
                        ? `${rule.windowSeconds}${t('secondsShort')}`
                        : `${rule.windowSeconds / 60}${t('minutesShort')}`}
                    </>
                  )}
                </strong>
                <span>
                  {rule.kind === 'listing' ? (
                    rule.listingExchanges?.length ? (
                      rule.listingExchanges.map(exchangeLabel).join(', ')
                    ) : (
                      t('allExternalExchanges')
                    )
                  ) : (
                    <>
                      $
                      {new Intl.NumberFormat(i18n.language, {
                        notation: 'compact',
                      }).format(rule.minQuoteVolume)}{' '}
                      / {t('hours24')}
                    </>
                  )}
                </span>
                <span>
                  {soundLabel(rule.sound)}
                  {rule.sound !== 'none' && ` · ${Math.round(rule.volume * 100)}%`}
                </span>
            </span>
            </span>
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
        rules={rules}
        sounds={sounds}
        onSaved={onSaved}
      />
    </>
  )
}
