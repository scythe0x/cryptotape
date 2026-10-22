import { useEffect, useState } from 'react'
import { desktop } from './api'

export function useClockTime(): number {
  const [timestamp, setTimestamp] = useState(Date.now)

  useEffect(() => {
    let stopped = false
    let offset = 0
    let syncing = false
    let lastSync = -Infinity
    let lastTick = Date.now()
    let lastMonotonic = performance.now()
    let timer = 0

    function schedule() {
      window.clearTimeout(timer)
      if (stopped) return
      const now = Date.now() + offset
      setTimestamp(now)
      timer = window.setTimeout(tick, Math.max(1, 1000 - (now % 1000)))
    }

    async function sync() {
      if (syncing || stopped) return
      syncing = true
      lastSync = Date.now()
      let best: { elapsed: number; offset: number } | undefined
      try {
        for (let attempt = 0; attempt < 3 && !stopped; attempt++) {
          const started = Date.now()
          const monotonic = performance.now()
          const sampleOffset = await desktop.clockOffset()
          const finished = Date.now()
          const elapsed = finished - started
          // discard samples spanning sleep or a local clock adjustment.
          if (elapsed < 0 || elapsed >= 4500 || Math.abs(elapsed - (performance.now() - monotonic)) >= 250) continue
          if (!best || elapsed < best.elapsed) {
            best = { elapsed, offset: sampleOffset }
          }
        }
      } catch {
        // keep a successful sample if a later probe failed.
      } finally {
        offset = best?.offset ?? 0
        syncing = false
        schedule()
      }
    }

    function tick() {
      const now = Date.now()
      const monotonic = performance.now()
      const elapsed = now - lastTick
      const discontinuity = elapsed > 2000 || elapsed < 0 || Math.abs(elapsed - (monotonic - lastMonotonic)) > 250
      lastTick = now
      lastMonotonic = monotonic
      if (discontinuity) offset = 0
      if (discontinuity || now - lastSync >= 60_000) void sync()
      schedule()
    }

    function resume() {
      if (!document.hidden) { offset = 0; tick(); void sync() }
    }

    schedule()
    void sync()
    document.addEventListener('visibilitychange', resume)
    window.addEventListener('pageshow', resume)
    return () => {
      stopped = true
      window.clearTimeout(timer)
      document.removeEventListener('visibilitychange', resume)
      window.removeEventListener('pageshow', resume)
    }
  }, [])

  return timestamp
}
