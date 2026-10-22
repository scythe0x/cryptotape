import { useEffect, useRef, useState } from 'react'
import { desktop, events, type Rule, type Settings, type SettingsSession } from './api'
import { applyLanguage } from './i18n'
import { localizeDiagnostic } from './i18n/diagnostics'
import { SettingsContent } from './components/SettingsContent'

export function SettingsWindow() {
  const [settings, setSettings] = useState<Settings | null>(null)
  const [rules, setRules] = useState<Rule[]>([])
  const [sounds, setSounds] = useState<string[]>([])
  const [session, setSession] = useState<SettingsSession | null>(null)
  const [error, setError] = useState('')
  const reload = useRef<() => void>(() => {})

  useEffect(() => {
    let active = true
    let request = 0
    let latestRevision = -1
    let rulesVersion = 0
    let settingsVersion = 0
    let soundsVersion = 0

    function refresh(opening?: SettingsSession) {
      const id = ++request
      const startedRulesVersion = rulesVersion
      const startedSettingsVersion = settingsVersion
      const startedSoundsVersion = soundsVersion
      void Promise.all([desktop.snapshot(), desktop.soundIds()])
        .then(([snapshot, soundIds]) => {
          if (!active || id !== request) return
          if (rulesVersion === startedRulesVersion) setRules(snapshot.rules)
          if (settingsVersion === startedSettingsVersion) setSettings(snapshot.settings)
          if (soundsVersion === startedSoundsVersion) setSounds(soundIds)
          if (opening) setSession(opening)
          setError('')
        })
        .catch((cause: unknown) => {
          if (active && id === request) setError(String(cause))
        })
    }

    function opened(value: SettingsSession) {
      if (!active || value.revision <= latestRevision) return
      latestRevision = value.revision
      setSession(null)
      refresh(value)
    }

    const stops = [
      events.settingsOpened(opened),
      events.rules((value) => { rulesVersion++; setRules(value) }),
      events.settings((value) => { settingsVersion++; setSettings(value) }),
      events.sounds((value) => { soundsVersion++; setSounds(value) }),
    ]
    reload.current = refresh
    void desktop.settingsSession().then(opened).catch((cause: unknown) => {
      if (active && latestRevision < 0) setError(String(cause))
    })
    return () => {
      active = false
      reload.current = () => {}
      stops.forEach((stop) => stop())
    }
  }, [])

  useEffect(() => {
    if (settings) applyLanguage(settings.language)
  }, [settings])

  function close() {
    void desktop.hideSettings().catch((cause: unknown) => setError(String(cause)))
  }

  if (!settings || !session) {
    return error ? <div className="inline-error settings-window-message" role="alert">{localizeDiagnostic(error)}</div> : null
  }

  return (
    <>
      <SettingsContent
        key={session.revision}
        section={session.section}
        settings={settings}
        rules={rules}
        sounds={sounds}
        onSaved={() => reload.current()}
        onClose={close}
      />
      {error && <div className="inline-error settings-window-error" role="alert">{localizeDiagnostic(error)}</div>}
    </>
  )
}
