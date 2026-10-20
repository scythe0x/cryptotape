import { Call, Events, Clipboard } from '@wailsio/runtime'
import { z } from 'zod'

export const terminalSchema = z.enum(['vataga', 'tiger'])
export type TerminalID = z.infer<typeof terminalSchema>

export const ruleSchema = z.object({
  id: z.number(),
  name: z.string(),
  percent: z.number(),
  windowSeconds: z.number(),
  sound: z.string(),
  color: z.string().regex(/^#[0-9a-fA-F]{6}$/),
  volume: z.number().min(0).max(1),
  minQuoteVolume: z.number().nonnegative(),
  terminals: z.array(terminalSchema),
})

export const languageSchema = z.enum(['en', 'ru'])

export const proxyProtocolSchema = z.enum(['http', 'https', 'socks5'])
export const proxySchema = z.object({
  id: z.string(),
  name: z.string(),
  host: z.string(),
  port: z.number(),
  protocol: proxyProtocolSchema,
  login: z.string(),
  password: z.string(),
})
export type ProxyProfile = z.infer<typeof proxySchema>

export const settingsSchema = z.object({
  language: languageSchema,
  proxies: z.array(proxySchema),
  activeProxyId: z.string(),
  cooldownSeconds: z.number(),
  monitorId: z.string(),
  tigerPort: z.number(),
  tigerLinkGroup: z.string(),
})

export const alertSchema = z.object({
  id: z.number(),
  ruleId: z.number(),
  ruleName: z.string(),
  symbol: z.string(),
  direction: z.enum(['up', 'down']),
  changePercent: z.number(),
  price: z.number(),
  spreadPercent: z.number(),
  quoteVolume: z.number(),
  eventTimeMs: z.number(),
  createdAtMs: z.number(),
  sound: z.string(),
  color: z.string(),
  volume: z.number(),
  terminals: z.array(terminalSchema),
  terminalStatus: z.string(),
  terminalError: z.string(),
})

export const healthSchema = z.object({
  viaProxy: z.boolean(),
  state: z.string(),
  detail: z.string(),
  connections: z.number(),
  symbols: z.number(),
  ready: z.number(),
  updatedAtMs: z.number(),
  pingMs: z.number().nullable(),
})

const snapshotSchema = z.object({
  rules: z.array(ruleSchema),
  settings: settingsSchema,
  health: healthSchema,
  alerts: z.array(alertSchema),
})

const monitorSchema = z.object({ id: z.string(), name: z.string(), primary: z.boolean() })
const alertUpdateSchema = z.object({
  id: z.number(),
  terminalStatus: z.string(),
  terminalError: z.string(),
})

export type Rule = z.infer<typeof ruleSchema>
export type Settings = z.infer<typeof settingsSchema>
export type Alert = z.infer<typeof alertSchema>
export type Health = z.infer<typeof healthSchema>
export type Snapshot = z.infer<typeof snapshotSchema>
export type Monitor = z.infer<typeof monitorSchema>
export type AlertUpdate = z.infer<typeof alertUpdateSchema>

async function invoke<T>(method: string, schema: z.ZodType<T>, ...args: unknown[]): Promise<T> {
  const result: unknown = await Call.ByName(`main.DesktopService.${method}`, ...args)
  return schema.parse(result)
}

async function command(method: string, ...args: unknown[]): Promise<void> {
  await Call.ByName(`main.DesktopService.${method}`, ...args)
}

export const desktop = {
  snapshot: () => invoke('GetSnapshot', snapshotSchema),
  saveRule: (rule: Rule) => invoke('SaveRule', ruleSchema, rule),
  deleteRule: (id: number) => command('DeleteRule', id),
  saveSettings: (settings: Settings) => command('SaveSettings', settings),
  saveProxy: (profile: ProxyProfile, use: boolean) => command('SaveProxy', profile, use),
  selectProxy: (id: string) => command('SelectProxy', id),
  deleteProxy: (id: string) => command('DeleteProxy', id),
  reconnect: () => command('Reconnect'),
  listAlerts: (before: number, limit = 100) => invoke('ListAlerts', z.array(alertSchema), before, limit),
  alertsAfter: (after: number, limit = 200) => invoke('AlertsAfter', z.array(alertSchema), after, limit),
  sessionAlerts: () => invoke('SessionAlerts', z.array(alertSchema)),
  openTerminal: (id: number, symbol: string, terminal: TerminalID) =>
    command('OpenTerminal', id, symbol, terminal),
  openTerminals: (id: number, symbol: string, terminals: TerminalID[]) =>
    invoke('OpenTerminals', z.object({ opened: z.number(), error: z.string() }), id, symbol, terminals),
  copySymbol: (symbol: string) => Clipboard.SetText(symbol),
  soundIds: () => invoke('SoundIDs', z.array(z.string())),
  importSound: (name: string, extension: string, encoded: string) =>
    invoke('ImportSound', z.string(), name, extension, encoded),
  previewSound: (id: string, volume: number) => command('PreviewSound', id, volume),
  monitors: () => invoke('Monitors', z.array(monitorSchema)),
  hideOverlay: (afterId: number) => command('HideOverlay', afterId),
}

export function listen<T>(name: string, schema: z.ZodType<T>, handle: (value: T) => void): () => void {
  return Events.On(name, (event) => {
    const parsed = schema.safeParse(event.data)
    if (parsed.success) handle(parsed.data)
  })
}

function alertBatches(handle: (alerts: Alert[], updates: AlertUpdate[]) => void): () => void {
  const pending = new Map<number, Alert>()
  const updates = new Map<number, AlertUpdate>()
  let frame = 0
  let fallback = 0

  function flush() {
    window.cancelAnimationFrame(frame)
    window.clearTimeout(fallback)
    frame = 0
    const alerts = [...pending.values()]
    const changes = [...updates.values()]
    pending.clear()
    updates.clear()
    handle(alerts, changes)
  }

  function schedule() {
    if (frame) return
    frame = window.requestAnimationFrame(flush)
    fallback = window.setTimeout(flush, 50)
  }

  const stopNew = listen('alert:new', alertSchema, (alert) => {
    pending.set(alert.id, alert)
    schedule()
  })
  const stopUpdate = listen('alert:update', alertUpdateSchema, (update) => {
    updates.set(update.id, update)
    schedule()
  })
  return () => {
    stopNew()
    stopUpdate()
    window.cancelAnimationFrame(frame)
    window.clearTimeout(fallback)
  }
}

export const events = {
  alerts: alertBatches,
  health: (handle: (health: Health) => void) => listen('health', healthSchema, handle),
  rules: (handle: (rules: Rule[]) => void) => listen('rules:changed', z.array(ruleSchema), handle),
  settings: (handle: (settings: Settings) => void) => listen('settings:changed', settingsSchema, handle),
  sounds: (handle: (sounds: string[]) => void) => listen('sounds:changed', z.array(z.string()), handle),
}
