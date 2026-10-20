import { useTranslation } from '../i18n'
import vataga from '../assets/vataga.svg'
import tiger from '../assets/tiger.ico'
import type { TerminalID } from '../api'

export const terminals: { id: TerminalID; name: string; icon: string }[] = [
  { id: 'vataga', name: 'Vataga', icon: vataga },
  { id: 'tiger', name: 'Tiger', icon: tiger },
]

type Props = { id: TerminalID; active?: boolean; toggle?: boolean; onClick: () => void; disabled?: boolean }

export function TerminalButton({ id, active = false, toggle = false, onClick, disabled }: Props) {
  const { t } = useTranslation()
  const terminal = terminals.find((item) => item.id === id)
  if (!terminal) return null
  return (
    <button
      type="button"
      className={`terminal-button ${toggle ? 'terminal-toggle' : ''} ${active ? 'is-active' : ''}`}
      aria-label={t(toggle ? 'autoOpenIn' : 'openIn', { terminal: terminal.name })}
      aria-pressed={toggle ? active : undefined}
      title={terminal.name}
      onClick={onClick}
      disabled={disabled}
    >
      <img src={terminal.icon} alt="" draggable={false} />
    </button>
  )
}
