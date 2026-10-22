export type CandleLabel = '' | '1 HOUR' | '4 HOURS' | '1 DAY'

const second = 1000
const minute = 60 * second
const hour = 60 * minute
const day = 24 * hour

export function candleWarning(timestamp: number): { yellow: boolean; label: CandleLabel } {
  const remaining = 5 * minute - timestamp % (5 * minute)
  const yellow = remaining <= 5 * second && Math.floor(timestamp / second) % 2 === 1
  let label: CandleLabel = ''
  if (day - timestamp % day <= 15 * second) label = '1 DAY'
  else if (4 * hour - timestamp % (4 * hour) <= 15 * second) label = '4 HOURS'
  else if (hour - timestamp % hour <= 15 * second) label = '1 HOUR'
  return { yellow, label }
}

export function timezoneLabel(offset: number): string {
  if (offset === 0) return 'UTC'
  const hours = Math.floor(Math.abs(offset) / 60)
  const minutes = Math.abs(offset) % 60
  return `UTC${offset < 0 ? '−' : '+'}${hours}${minutes ? `:${String(minutes).padStart(2, '0')}` : ''}`
}

export const clockTimezones = Array.from({ length: 105 }, (_, index) => {
  const offset = -720 + index * 15
  return { value: `UTC${offset}`, label: timezoneLabel(offset) }
})

export function clockDisplay(timestamp: number, timezone: string): { time: string; zone: string } {
  const date = new Date(timestamp)
  const offset = timezone === 'system' ? -date.getTimezoneOffset() : Number(timezone.slice(3))
  const shifted = new Date(timestamp + offset * minute)
  const time = [shifted.getUTCHours(), shifted.getUTCMinutes(), shifted.getUTCSeconds()]
    .map((part) => String(part).padStart(2, '0')).join(':')
  return { time, zone: timezoneLabel(offset) }
}

