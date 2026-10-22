import { useEffect, useState } from 'react'
import { Window } from '@wailsio/runtime'
import { X } from 'lucide-react'
import { desktop } from './api'
import { candleWarning, clockDisplay, clockTimezones } from './clock'
import { useClockTime } from './useClockTime'
import { useTranslation, applyLanguage } from './i18n'
import './clock.css'

export function ClockWindow() {
  const { t } = useTranslation()
  const [timezone, setTimezone] = useState('system')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const timestamp = useClockTime()
  const display = clockDisplay(timestamp, timezone)
  const warning = candleWarning(timestamp)

  useEffect(() => {
    let active = true
    void desktop.clockTimezone().then((zone) => { if (active) setTimezone(zone) })
      .catch((cause: unknown) => { if (active) setError(String(cause)) })
    void desktop.snapshot().then((snapshot) => { if (active) applyLanguage(snapshot.settings.language) })
      .catch((cause: unknown) => { if (active) setError(String(cause)) })
    return () => { active = false }
  }, [])

  async function changeTimezone(zone: string) {
    setSaving(true)
    try {
      await desktop.setClockTimezone(zone)
      setTimezone(zone)
      setError('')
    } catch (cause) {
      setError(String(cause))
    } finally {
      setSaving(false)
    }
  }

  return (
    <main className="candle-clock">
      <svg className="clock-composition" viewBox="0 0 228 78" aria-label={display.time} role="img">
        <text className={warning.yellow ? 'clock-time clock-yellow' : 'clock-time'} x="0" y="41">{display.time}</text>
        <text className="clock-label" x="0" y="75">{warning.label}</text>
      </svg>
      <div className="clock-controls">
        <label className="clock-zone" title={error || t('clockTimezone')}>
          <span>{display.zone}{error ? ' !' : ''}</span>
          <select aria-label={t('clockTimezone')} value={timezone} disabled={saving}
            onChange={(event) => void changeTimezone(event.target.value)}>
            <option value="system">{t('systemTimezone')}</option>
            {clockTimezones.map((zone) => <option key={zone.value} value={zone.value}>{zone.label}</option>)}
          </select>
        </label>
        <button type="button" aria-label={t('close')} title={t('close')}
          onClick={() => void Window.Close().catch((cause: unknown) => setError(String(cause)))}>
          <X size={13} />
        </button>
      </div>
    </main>
  )
}
