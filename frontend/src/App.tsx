import { Feed } from "@/components/Feed"
import { useAlertSound } from "@/hooks/useAlertSound"
import { alertToFeedEvent, useFeed } from "@/hooks/useFeed"
import { useWebSocket } from "@/hooks/useWebSocket"
import type { PriceImpulseAlert } from "@/types"
import { useCallback, useEffect, useMemo, useState } from "react"

const WS_URL = "ws://localhost:8080/ws"
const FEED_FILTER_KEY = "cryptotape:min-visible-pct"
const CHARTS_OPEN_KEY = "cryptotape:charts-open"

function readInitialMinVisiblePct() {
  if (typeof window === "undefined") return 0

  const raw = window.localStorage.getItem(FEED_FILTER_KEY)
  const parsed = raw === null ? 0 : Number.parseFloat(raw)
  return Number.isFinite(parsed) ? parsed : 0
}

function readInitialChartsOpen() {
  if (typeof window === "undefined") return true

  const raw = window.localStorage.getItem(CHARTS_OPEN_KEY)
  return raw === null ? true : raw === "1"
}

export function App() {
  const { items, upsert, clear } = useFeed()
  const { muted, toggleMuted, playNewAlert } = useAlertSound()
  const [minVisiblePct, setMinVisiblePct] = useState(readInitialMinVisiblePct)
  const [chartsOpen, setChartsOpen] = useState(readInitialChartsOpen)

  const visibleItems = useMemo(
    () =>
      items.filter((item) => {
        if (item.type !== "PRICE_IMPULSE") return true
        const alert = item.data as PriceImpulseAlert
        return Math.abs(alert.maxPtc) >= minVisiblePct
      }),
    [items, minVisiblePct]
  )

  useEffect(() => {
    window.localStorage.setItem(FEED_FILTER_KEY, String(minVisiblePct))
  }, [minVisiblePct])

  useEffect(() => {
    window.localStorage.setItem(CHARTS_OPEN_KEY, chartsOpen ? "1" : "0")
  }, [chartsOpen])

  const onMessage = useCallback(
    (raw: string) => {
      try {
        const alert = JSON.parse(raw) as PriceImpulseAlert
        const mutation = upsert(alertToFeedEvent(alert))
        if (
          mutation === "inserted" &&
          Math.abs(alert.maxPtc) >= minVisiblePct
        ) {
          void playNewAlert()
        }
      } catch {
        console.warn("unparseable ws message", raw)
      }
    },
    [minVisiblePct, playNewAlert, upsert]
  )

  const wsState = useWebSocket(WS_URL, { onMessage })

  return (
    <Feed
      items={visibleItems}
      wsState={wsState}
      muted={muted}
      minVisiblePct={minVisiblePct}
      chartsOpen={chartsOpen}
      onToggleMute={toggleMuted}
      onToggleCharts={() => setChartsOpen((prev) => !prev)}
      onMinVisiblePctChange={setMinVisiblePct}
      onClear={clear}
    />
  )
}

export default App
