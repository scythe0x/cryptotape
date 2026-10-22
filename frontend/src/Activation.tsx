import { useCallback, useEffect, useRef, useState } from 'react'
import { Browser, Call, Events } from '@wailsio/runtime'
import { z } from 'zod'
import background from './assets/activation.png'
import './activation.css'

const stateSchema = z.object({
  phase: z.enum(['loading', 'locked', 'ready']),
  code: z.string().default(''),
  message: z.string().default(''),
  botUrl: z.string(),
  canRetry: z.boolean().default(false),
})
type State = z.infer<typeof stateSchema>
const initial: State = { phase: 'loading', code: '', message: '', botUrl: 'https://t.me/mythrascalping_bot', canRetry: false }

async function stateCall(method: string, ...args: unknown[]): Promise<State> {
  const value: unknown = await Call.ByName(`main.ActivationService.${method}`, ...args)
  return stateSchema.parse(value)
}

export function Activation() {
  const [state, setState] = useState(initial)
  const [key, setKey] = useState('')
  const [busy, setBusy] = useState(false)
  const [progress, setProgress] = useState(0)
  const opening = useRef(false)
  const submitted = useRef(false)
  const painted = useRef(false)
  const stateVersion = useRef(0)

  const applyState = useCallback((value: State) => {
    stateVersion.current++
    setState(value)
  }, [])

  useEffect(() => {
    let stopped = false
    let polling = false
    const start = performance.now()
    const stop = Events.On('license:state', (event) => {
      if (!stopped) applyState(stateSchema.parse(event.data))
    })
    const interval = window.setInterval(() => {
      if (opening.current) return
      setProgress(Math.min(94, (performance.now() - start) / 3000 * 94))
      if (polling || submitted.current) return
      polling = true
      const version = stateVersion.current
      const current = () => !stopped && !submitted.current && version === stateVersion.current
      void stateCall('State').then((value) => {
        if (current()) applyState(value)
      }).catch(() => {
        if (current()) applyState({ ...initial, phase: 'locked', message: 'Не удалось проверить доступ' })
      }).finally(() => { polling = false })
    }, 250)
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') void Call.ByName('main.ActivationService.Quit')
    }
    window.addEventListener('keydown', onKey)
    return () => {
      stopped = true
      stop()
      window.clearInterval(interval)
      window.removeEventListener('keydown', onKey)
    }
  }, [applyState])

  useEffect(() => {
    if (state.phase !== 'ready') { opening.current = false; return }
    if (opening.current) return
    opening.current = true
    void Call.ByName('main.ActivationService.Continue').then(() => {
      setProgress(100)
      setKey('')
    }).catch(() => {
      opening.current = false
      applyState({ ...initial, phase: 'locked', message: 'Доступ к приложению закрыт' })
    })
  }, [state.phase, applyState])

  function firstPaint() {
    if (painted.current) return
    painted.current = true
    requestAnimationFrame(() => requestAnimationFrame(() => {
      void Call.ByName('main.ActivationService.FirstPaint').catch(() => {
        applyState({ ...initial, phase: 'locked', message: 'Не удалось запустить приложение' })
      })
    }))
  }

  const locked = state.phase === 'locked'
  async function activate() {
    if (submitted.current || (!key.trim() && !state.canRetry)) return
    submitted.current = true
    const version = ++stateVersion.current
    setBusy(true)
    try {
      const value = key.trim() ? await stateCall('Activate', key) : await stateCall('Retry')
      if (version === stateVersion.current) applyState(value)
    } catch {
      if (version === stateVersion.current) applyState({ ...initial, phase: 'locked', message: 'Сервер недоступен' })
    }
    finally { submitted.current = false; setBusy(false) }
  }

  return <main className={`activation${locked || busy ? ' activation-locked' : ''}`}>
    <img className="activation-background" src={background} alt="Mythra" draggable={false} onLoad={firstPaint} />
    {(locked || busy) && <section className="activation-shade">
      <form className="activation-form" onSubmit={(event) => { event.preventDefault(); void activate() }}>
        <h1>Введите ключ</h1>
        <p>Ключ можно получить в <button className="activation-link" type="button" onClick={() => { void Browser.OpenURL(initial.botUrl) }}>Telegram-боте Mythra</button></p>
        <label className="activation-label" htmlFor="activation-key">Ключ доступа</label>
        <input id="activation-key" autoFocus autoComplete="off" spellCheck={false} value={key} maxLength={64} disabled={busy}
          placeholder="XXXXXXXX-XXXXXXXX-XXXXXXXX-XXXXXXXX" onChange={(event) => setKey(event.target.value)} />
        <button className="activation-submit" disabled={busy || (!key.trim() && !state.canRetry)}>{busy ? 'Проверяем…' : key.trim() || !state.canRetry ? 'Войти' : 'Повторить'}</button>
        <p className="activation-error" role="alert">{state.message || '\u00a0'}</p>
      </form>
    </section>}
    {!locked && !busy && <div className="activation-progress" role="progressbar" aria-label="Загрузка" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(progress)} style={{ width: `${progress}%` }} />}
  </main>
}
