import { useEffect, useRef, useState } from 'react'
import { Clock3, Settings2 } from 'lucide-react'
import { desktop, events } from './api'
import { WindowTitleBar } from './components/WindowTitleBar'
import { SoundToggle } from './components/SoundToggle'
import { ConnectionStatus } from './components/ConnectionStatus'
import { AlertsTable } from './components/AlertsTable'
import { Button } from './components/ui'
import { useTranslation, applyLanguage } from './i18n'
import { localizeDiagnostic } from './i18n/diagnostics'
import { useAppStore } from './store'
import { useMainWindowMinimum } from './useMainWindowMinimum'
import { defaultTableColumns, type SettingsSection } from './api'

export function App() {
  const { t } = useTranslation()
  const rules = useAppStore((state) => state.rules)
  const settings = useAppStore((state) => state.settings)
  const health = useAppStore((state) => state.health)
  const alerts = useAppStore((state) => state.alerts)
  const error = useAppStore((state) => state.error)
  const setSnapshot = useAppStore((state) => state.setSnapshot)
  const setRules = useAppStore((state) => state.setRules)
  const setSettings = useAppStore((state) => state.setSettings)
  const setHealth = useAppStore((state) => state.setHealth)
  const addAlerts = useAppStore((state) => state.addAlerts)
  const setError = useAppStore((state) => state.setError)
  const shellRef = useRef<HTMLElement>(null)
  useMainWindowMinimum(shellRef, settings?.tableColumns ?? defaultTableColumns, setError)
  const [hasMore, setHasMore] = useState(false)
  const loadingMore = useRef(false)
  const language = settings?.language
  useEffect(() => {
    if (language) applyLanguage(language)
  }, [language])

  useEffect(() => {
    let stopped = false
    let cursor = 0
    let polling = false
    const unsubscribers = [
      events.alerts(addAlerts),
      events.health(setHealth),
      events.rules(setRules),
      events.settings(setSettings),
    ]
    const initial = desktop.snapshot()
    void initial
      .then((snapshot) => {
        if (stopped) return
        setSnapshot(snapshot)
        setHasMore(snapshot.alerts.length === 100)
        cursor = snapshot.alerts[0]?.id ?? 0
      })
      .catch((cause: unknown) => setError(String(cause)))

    const interval = window.setInterval(async () => {
      if (stopped || polling) return
      polling = true
      try {
        for (let page = 0; page < 10; page++) {
          const fresh = await desktop.alertsAfter(cursor)
          if (stopped || fresh.length === 0) break
          addAlerts(fresh)
          cursor = fresh[fresh.length - 1].id
          if (fresh.length < 200) break
        }
      } catch (cause) {
        setError(`alert sync: ${String(cause)}`)
      } finally {
        polling = false
      }
    }, 2000)
    return () => {
      stopped = true
      window.clearInterval(interval)
      unsubscribers.forEach((stop) => stop())
    }
  }, [addAlerts, setError, setHealth, setRules, setSettings, setSnapshot])

  function openSettings(section: SettingsSection) {
    void desktop.openSettings(section).catch((cause: unknown) => setError(String(cause)))
  }

  async function loadMore() {
    if (!hasMore || loadingMore.current) return
    const oldest = useAppStore.getState().alerts.at(-1)?.id
    if (!oldest) return
    loadingMore.current = true
    try {
      const older = await desktop.listAlerts(oldest)
      addAlerts(older)
      setHasMore(older.length === 100)
    } catch (cause) {
      setError(String(cause))
    } finally {
      loadingMore.current = false
    }
  }

  return (
    <main className="app-shell" ref={shellRef}>
      <WindowTitleBar
        status={<ConnectionStatus health={health} />}
        onError={setError}
        actions={<>
          <SoundToggle settings={settings} onError={setError} />
          <Button size="icon" variant="ghost" aria-label={t('openClock')} title={t('openClock')}
            onClick={() => void desktop.openClock().catch((cause: unknown) => setError(String(cause)))}>
            <Clock3 size={15} />
          </Button>
          <Button
            size="icon"
            variant="ghost"
            aria-label={t('settings')}
            title={t('settings')}
            onClick={() => openSettings('alerts')}
          >
            <Settings2 size={15} />
          </Button>
        </>}
      />
      {error && (
        <div className="error-banner" title={error} onClick={() => setError('')}>
          {localizeDiagnostic(error)}
          <span>×</span>
        </div>
      )}
      <AlertsTable
        alerts={alerts}
        columns={settings?.tableColumns}
        rules={rules}
        hasMore={hasMore}
        loadMore={() => void loadMore()}
        onError={setError}
      />
    </main>
  )
}
