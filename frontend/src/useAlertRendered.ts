import { useEffect, useRef } from 'react'
import { desktop } from './api'

// this acknowledges a browser frame, not physical presentation by the OS.
export function useAlertRendered(ids: number[], view: 'feed' | 'overlay') {
  const acknowledged = useRef(new Set<number>())
  const key = ids.join(',')
  useEffect(() => {
    const frame = window.requestAnimationFrame(() => {
      for (const id of key.split(',').filter(Boolean).map(Number)) {
        if (acknowledged.current.has(id)) continue
        if (acknowledged.current.size >= 8192) acknowledged.current.clear()
        acknowledged.current.add(id)
        void desktop.alertRendered(id, view).catch(() => acknowledged.current.delete(id))
      }
    })
    return () => window.cancelAnimationFrame(frame)
  }, [key, view])
}
