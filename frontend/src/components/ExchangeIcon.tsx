import { CircleHelp } from 'lucide-react'
import { exchangeIcons } from '../exchange-icons'
import { exchangeLabel } from '../listings'

export function ExchangeIcon({ exchange = 'binance' }: { exchange?: string }) {
  const icon = exchangeIcons.get(exchange)
  const name = exchangeLabel(exchange)
  if (!icon) return <CircleHelp className="exchange-icon" role="img" aria-label={name} />
  return <img className="exchange-icon" src={icon} alt={name} draggable={false} />
}
