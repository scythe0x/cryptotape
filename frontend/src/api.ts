import { Call, Events } from '@wailsio/runtime'
import { z } from 'zod'

export const ruleSchema = z.object({
  id: z.number(),
  name: z.string(),
  percent: z.number(),
  windowSeconds: z.number(),
  sound: z.string(),
  openTerminal: z.boolean(),
  enabled: z.boolean(),
})

export const settingsSchema = z.object({
  proxy: z.string(),
  minQuoteVolume: z.number(),
  cooldownSeconds: z.number(),
  monitorId: z.string(),
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
  openTerminal: z.boolean(),
  terminalStatus: z.string(),
  terminalError: z.string(),
})

export const healthSchema = z.object({
  state: z.string(),
  detail: z.string(),
  connections: z.number(),
  symbols: z.number(),
  ready: z.number(),
  updatedAtMs: z.number(),
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
  testProxy: (proxy: string) => command('TestProxy', proxy),
  listAlerts: (before: number, limit = 100) => invoke('ListAlerts', z.array(alertSchema), before, limit),
  alertsAfter: (after: number, limit = 200) => invoke('AlertsAfter', z.array(alertSchema), after, limit),
  sessionAlerts: () => invoke('SessionAlerts', z.array(alertSchema)),
  openTerminal: (id: number, symbol: string) => command('OpenTerminal', id, symbol),
  soundIds: () => invoke('SoundIDs', z.array(z.string())),
  importSound: (name: string, extension: string, encoded: string) =>
    invoke('ImportSound', z.string(), name, extension, encoded),
  previewSound: (id: string) => command('PreviewSound', id),
  monitors: () => invoke('Monitors', z.array(monitorSchema)),
  hideOverlay: () => command('HideOverlay'),
}

export function listen<T>(name: string, schema: z.ZodType<T>, handle: (value: T) => void): () => void {
  return Events.On(name, (event) => {
    const parsed = schema.safeParse(event.data)
    if (parsed.success) handle(parsed.data)
  })
}

export const events = {
  alert: (handle: (alert: Alert) => void) => listen('alert:new', alertSchema, handle),
  update: (handle: (update: AlertUpdate) => void) => listen('alert:update', alertUpdateSchema, handle),
  health: (handle: (health: Health) => void) => listen('health', healthSchema, handle),
  rules: (handle: (rules: Rule[]) => void) => listen('rules:changed', z.array(ruleSchema), handle),
  settings: (handle: (settings: Settings) => void) => listen('settings:changed', settingsSchema, handle),
  sounds: (handle: (sounds: string[]) => void) => listen('sounds:changed', z.array(z.string()), handle),
}
