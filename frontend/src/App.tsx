import { useEffect, useRef, useState } from 'react'
import { Search, Settings2, ShieldCheck, WifiOff } from 'lucide-react'
import { desktop, events } from './api'
import { AlertsTable } from './components/AlertsTable'
import { SettingsDialog } from './components/SettingsDialog'
import { Button, Input } from './components/ui'
import { useAppStore } from './store'

export function App() {
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
  const updateAlert = useAppStore((state) => state.updateAlert)
  const setError = useAppStore((state) => state.setError)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [search, setSearch] = useState('')
  const [sounds, setSounds] = useState<string[]>(['bell', 'chime', 'notification', 'alarm', 'beep'])
  const [hasMore, setHasMore] = useState(false)
  const loadingMore = useRef(false)

  useEffect(() => {
    let stopped = false
    let cursor = 0
    let polling = false
    const unsubscribers = [
      events.alert((alert) => addAlerts([alert])),
      events.update(updateAlert),
      events.health(setHealth),
      events.rules(setRules),
      events.settings(setSettings),
      events.sounds(setSounds),
    ]
    void desktop.snapshot().then((snapshot) => {
      if (stopped) return
      setSnapshot(snapshot)
      setHasMore(snapshot.alerts.length === 100)
      cursor = snapshot.alerts[0]?.id ?? 0
    }).catch((cause: unknown) => setError(String(cause)))
    void desktop.soundIds().then(setSounds).catch((cause: unknown) => setError(String(cause)))

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
    return () => { stopped = true; window.clearInterval(interval); unsubscribers.forEach((stop) => stop()) }
  }, [addAlerts, setError, setHealth, setRules, setSettings, setSnapshot, updateAlert])

  async function refresh() {
    try {
      setSnapshot(await desktop.snapshot())
    } catch (cause) {
      setError(String(cause))
    }
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

  return <main className="app-shell">
    <div className="connection-bar">
      <div role="status" className={`connection-pill connection-${health?.state ?? 'starting'}`} title={health?.detail ?? 'connecting'}>
        {health?.state === 'connected' ? <ShieldCheck size={13} /> : <WifiOff size={13} />}
        {health?.state ?? 'starting'}
      </div>
      <div className="search-box"><Search size={13} /><Input aria-label="search alerts" value={search}
        onChange={(event) => setSearch(event.target.value)} placeholder="search" /></div>
      <Button size="icon" variant="ghost" aria-label="settings" title="settings" onClick={() => setSettingsOpen(true)}><Settings2 size={15} /></Button>
    </div>
    {error && <div className="error-banner" onClick={() => setError('')}>{error}<span>×</span></div>}
    {health && ['error', 'reconnecting', 'degraded'].includes(health.state) && <div className="connection-banner">
      <WifiOff size={14} /><span>{health.detail}</span><Button variant="ghost" onClick={() => setSettingsOpen(true)}>settings</Button>
    </div>}
    <AlertsTable alerts={alerts} search={search} hasMore={hasMore} loadMore={() => void loadMore()} onError={setError} />

    {settings && settingsOpen && <SettingsDialog open={settingsOpen} onOpenChange={setSettingsOpen} settings={settings} rules={rules} sounds={sounds}
      onSaved={() => void refresh()} onImported={() => void desktop.soundIds().then(setSounds)} />}
  </main>
}
