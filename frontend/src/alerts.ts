import type { Alert, AlertUpdate } from './api'

export function mergeAlerts(current: Alert[], incoming: Alert[], updates: AlertUpdate[] = []): Alert[] {
  const byId = new Map(current.map((alert) => [alert.id, alert]))
  for (const alert of incoming) {
    const previous = byId.get(alert.id)
    const unresolved = alert.terminalStatus === 'pending' || alert.terminalStatus === 'skipped'
    const keepTerminalResult = previous && unresolved && previous.terminalStatus !== 'pending' && previous.terminalStatus !== 'skipped'
    byId.set(alert.id, keepTerminalResult ? {
      ...alert, terminalStatus: previous.terminalStatus, terminalError: previous.terminalError,
    } : alert)
  }
  for (const update of updates) {
    const alert = byId.get(update.id)
    if (alert) byId.set(update.id, { ...alert, ...update })
  }
  return [...byId.values()].sort((a, b) => b.id - a.id)
}

export function notificationCards(current: Alert[], incoming: Alert[]): Alert[] {
  return mergeAlerts(current, incoming).slice(0, 3).reverse()
}
