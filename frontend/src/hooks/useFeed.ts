import { startTransition, useCallback, useRef, useState } from "react"
import type {
  FeedEvent,
  FeedEventType,
  FeedMutation,
  PriceImpulseAlert,
} from "@/types"

// Items are ordered: newest first, position stable after insert.
// On update: only data + updatedAt change — item stays in its row.

export function useFeed() {
  const [items, setItems] = useState<FeedEvent[]>([])
  const itemsRef = useRef<FeedEvent[]>([])

  const upsert = useCallback((incoming: FeedEvent): FeedMutation => {
    const prev = itemsRef.current
    const idx = prev.findIndex((item) => item.id === incoming.id)

    let next: FeedEvent[]
    let mutation: FeedMutation

    if (idx === -1) {
      mutation = "inserted"
      next = [incoming, ...prev]
    } else {
      mutation = "updated"
      next = [...prev]
      next[idx] = { ...incoming, insertedAt: prev[idx].insertedAt }
    }

    itemsRef.current = next
    startTransition(() => setItems(next))

    return mutation
  }, [])

  const clear = useCallback(() => {
    itemsRef.current = []
    startTransition(() => setItems([]))
  }, [])

  return { items, upsert, clear }
}

// ─── Factory helpers ────────────────────────────────────────────────────────

export function alertToFeedEvent(
  alert: PriceImpulseAlert
): FeedEvent<PriceImpulseAlert> {
  const now = Date.now()
  return {
    id: alert.impulseId,
    type: "PRICE_IMPULSE" as FeedEventType,
    insertedAt: now,
    updatedAt: now,
    data: alert,
  }
}
