import { create } from 'zustand'
import type { Alert, AlertUpdate, Health, Rule, Settings, Snapshot } from './api'

type AppState = {
  rules: Rule[]
  settings: Settings | null
  health: Health | null
  alerts: Alert[]
  error: string
  setSnapshot: (snapshot: Snapshot) => void
  setRules: (rules: Rule[]) => void
  setSettings: (settings: Settings) => void
  setHealth: (health: Health) => void
  addAlerts: (alerts: Alert[]) => void
  updateAlert: (update: AlertUpdate) => void
  setError: (error: string) => void
}

export const useAppStore = create<AppState>((set) => ({
  rules: [],
  settings: null,
  health: null,
  alerts: [],
  error: '',
  setSnapshot: (snapshot) => set((state) => ({
    rules: snapshot.rules,
    settings: snapshot.settings,
    health: snapshot.health,
    alerts: [...new Map([...snapshot.alerts, ...state.alerts].map((alert) => [alert.id, alert])).values()]
      .sort((a, b) => b.id - a.id),
  })),
  setRules: (rules) => set({ rules }),
  setSettings: (settings) => set({ settings }),
  setHealth: (health) => set({ health }),
  addAlerts: (incoming) => set((state) => {
    const byId = new Map(state.alerts.map((alert) => [alert.id, alert]))
    for (const alert of incoming) byId.set(alert.id, alert)
    return { alerts: [...byId.values()].sort((a, b) => b.id - a.id) }
  }),
  updateAlert: (update) => set((state) => ({
    alerts: state.alerts.map((alert) => alert.id === update.id ? { ...alert, ...update } : alert),
  })),
  setError: (error) => set({ error }),
}))
