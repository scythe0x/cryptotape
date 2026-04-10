import { useCallback, useEffect, useRef, useState } from "react"
import type { WSState, WSStatus } from "@/types"

const RECONNECT_DELAY_MS = 3000
const PING_INTERVAL_MS = 5000

interface Options {
  onMessage: (raw: string) => void
}

interface WSPongMessage {
  type: "pong"
  clientTs: number
  serverTs: number
}

function isPongMessage(value: unknown): value is WSPongMessage {
  if (!value || typeof value !== "object") return false

  const message = value as Partial<WSPongMessage>
  return (
    message.type === "pong" &&
    typeof message.clientTs === "number" &&
    typeof message.serverTs === "number"
  )
}

function toHealthUrl(wsUrl: string) {
  if (typeof window === "undefined") return null

  try {
    const url = new URL(wsUrl, window.location.href)
    url.protocol = url.protocol === "wss:" ? "https:" : "http:"
    url.pathname = "/healthz"
    url.search = ""
    url.hash = ""
    return url.toString()
  } catch {
    return null
  }
}

export function useWebSocket(url: string, { onMessage }: Options): WSState {
  const [status, setStatus] = useState<WSStatus>("connecting")
  const [pingMs, setPingMs] = useState<number | null>(null)
  const [lastPongAt, setLastPongAt] = useState<number | null>(null)

  const wsRef = useRef<WebSocket | null>(null)
  const onMsgRef = useRef(onMessage)
  const pingTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const unmounted = useRef(false)

  useEffect(() => {
    onMsgRef.current = onMessage
  }, [onMessage])

  const stopPingLoop = useCallback(() => {
    if (pingTimerRef.current) {
      clearInterval(pingTimerRef.current)
      pingTimerRef.current = null
    }
  }, [])

  const sendPing = useCallback((ws: WebSocket) => {
    if (ws.readyState !== WebSocket.OPEN) return
    ws.send(JSON.stringify({ type: "ping", clientTs: Date.now() }))
  }, [])

  const measureFallbackPing = useCallback(async () => {
    const healthUrl = toHealthUrl(url)
    if (!healthUrl) return

    try {
      const startedAt = performance.now()
      const response = await fetch(healthUrl, {
        method: "GET",
        cache: "no-store",
      })
      if (!response.ok) return

      setPingMs(Math.round(performance.now() - startedAt))
    } catch {
      // keep previous ping value if health probe fails
    }
  }, [url])

  useEffect(() => {
    unmounted.current = false
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null

    const connect = () => {
      if (unmounted.current) return
      setStatus("connecting")

      const ws = new WebSocket(url)
      let reconnectScheduled = false
      wsRef.current = ws

      const scheduleReconnect = () => {
        if (reconnectScheduled || unmounted.current) return
        reconnectScheduled = true

        stopPingLoop()
        setStatus("disconnected")
        setPingMs(null)
        setLastPongAt(null)

        if (reconnectTimer) clearTimeout(reconnectTimer)
        reconnectTimer = setTimeout(connect, RECONNECT_DELAY_MS)
      }

      ws.onopen = () => {
        if (unmounted.current) return

        setStatus("connected")
        setPingMs(null)
        setLastPongAt(null)

        stopPingLoop()
        sendPing(ws)
        pingTimerRef.current = setInterval(() => sendPing(ws), PING_INTERVAL_MS)
        void measureFallbackPing()
      }

      ws.onmessage = (event) => {
        if (typeof event.data !== "string") return

        try {
          const parsed = JSON.parse(event.data) as unknown
          if (isPongMessage(parsed)) {
            setPingMs(Math.max(0, Date.now() - parsed.clientTs))
            setLastPongAt(Date.now())
            return
          }
        } catch {
          // non-control frames are forwarded to the app
        }

        onMsgRef.current(event.data)
      }

      ws.onclose = scheduleReconnect
      ws.onerror = () => {
        ws.close()
        scheduleReconnect()
      }
    }

    connect()

    return () => {
      unmounted.current = true
      stopPingLoop()
      wsRef.current?.close()

      if (reconnectTimer) clearTimeout(reconnectTimer)
    }
  }, [measureFallbackPing, sendPing, stopPingLoop, url])

  useEffect(() => {
    if (status !== "connected") return
    if (lastPongAt !== null) return

    const kickoff = window.setTimeout(() => {
      void measureFallbackPing()
    }, 0)
    const timer = window.setInterval(() => {
      void measureFallbackPing()
    }, PING_INTERVAL_MS)

    return () => {
      window.clearTimeout(kickoff)
      window.clearInterval(timer)
    }
  }, [lastPongAt, measureFallbackPing, status])

  return { status, pingMs, lastPongAt }
}
