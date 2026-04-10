// ─── Alert from backend ────────────────────────────────────────────────────

export interface PriceImpulseAlert {
  impulseId: string
  symbol: string
  direction: "up" | "down"
  currentPct: number
  maxPtc: number
  durationMs: number
  startPrice: number
  endPrice: number
  timestamp: string
}

// ─── Feed event envelope ───────────────────────────────────────────────────
// Extend FeedEventType and FeedEventData as new event kinds are added.

export type FeedEventType =
  | "PRICE_IMPULSE"
  | "OPEN_INTEREST"
  | "VOLUME_SPIKE"
  | "NEWS"

export interface FeedEvent<T = unknown> {
  id: string // impulseId or generated id — used for upsert
  type: FeedEventType
  insertedAt: number // Date.now() of first insert — drives slide-in animation
  updatedAt: number // Date.now() of last update — drives flash animation
  data: T
}

export type PriceImpulseFeedEvent = FeedEvent<PriceImpulseAlert>
export type FeedMutation = "inserted" | "updated"

// ─── WebSocket ─────────────────────────────────────────────────────────────

export type WSStatus = "connecting" | "connected" | "disconnected"

export interface WSState {
  status: WSStatus
  pingMs: number | null
  lastPongAt: number | null
}
