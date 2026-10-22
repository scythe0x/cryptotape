import { useEffect, useState, type ReactNode } from 'react'
import { Call, Events } from '@wailsio/runtime'
import { z } from 'zod'

const stateSchema = z.object({ phase: z.enum(['loading', 'locked', 'ready']) })

export function LicenseBoundary({ children }: { children: ReactNode }) {
  const [ready, setReady] = useState(false)
  useEffect(() => {
    let stopped = false
    let changed = false
    const stop = Events.On('license:state', (event) => {
      const state = stateSchema.safeParse(event.data)
      changed = true
      setReady(state.success && state.data.phase === 'ready')
    })
    void Call.ByName('main.ActivationService.State').then((value: unknown) => {
      if (stopped || changed) return
      const state = stateSchema.safeParse(value)
      setReady(state.success && state.data.phase === 'ready')
    }).catch(() => { if (!stopped) setReady(false) })
    return () => { stopped = true; stop() }
  }, [])
  return ready ? children : null
}
