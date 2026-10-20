import { create } from 'zustand'
import type { Alert, AlertUpdate, Health, Rule, Settings, Snapshot } from './api'
import { mergeAlerts } from './alerts'

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
  addAlerts: (alerts: Alert[], updates?: AlertUpdate[]) => void
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
    alerts: mergeAlerts(state.alerts, snapshot.alerts),
  })),
  setRules: (rules) => set({ rules }),
  setSettings: (settings) => set({ settings }),
  setHealth: (health) => set({ health }),
  addAlerts: (incoming, updates = []) => set((state) => ({ alerts: mergeAlerts(state.alerts, incoming, updates) })),
  setError: (error) => set({ error }),
}))
