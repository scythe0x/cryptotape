import type { Settings } from '../api'
import { events } from '../api'
import { applyLanguage } from './index'

// each webview has its own language instance; SQLite remains the source of truth.
export function syncLanguage(initial: Promise<Settings>, onError: (error: unknown) => void): () => void {
  let stopped = false
  let changed = false
  const unsubscribe = events.settings((settings) => {
    changed = true
    applyLanguage(settings.language)
  })
  void initial
    .then((settings) => {
      if (!stopped && !changed) applyLanguage(settings.language)
    })
    .catch((error: unknown) => {
      if (!stopped) onError(error)
    })
  return () => {
    stopped = true
    unsubscribe()
  }
}
