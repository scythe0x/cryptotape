package server

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/whoiscyberial/cryptotape/backend/types"
)

const (
	// clientSendBuf — буфер на клиента. Если клиент не успевает читать
	// и буфер заполнен, он отключается, не блокируя остальных.
	clientSendBuf = 256
	writeTimeout  = 2 * time.Second
)

type client struct {
	conn *websocket.Conn
	send chan []byte
	done chan struct{}
	once sync.Once
}

// Hub manages active WebSocket connections and broadcasts alerts to all of them.
// Each client gets its own goroutine and buffered channel, so a slow client
// never blocks the broadcast loop — critical under 1000+ ticker load.
type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*client]struct{})}
}

func (h *Hub) add(c *client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	_, exists := h.clients[c]
	if exists {
		delete(h.clients, c)
	}
	h.mu.Unlock()

	if exists {
		c.once.Do(func() {
			close(c.done)
		})
	}
}

// Register upgrades an HTTP connection to WebSocket, registers the client,
// and starts its write pump.
func (h *Hub) Register(conn *websocket.Conn) *client {
	c := &client{
		conn: conn,
		send: make(chan []byte, clientSendBuf),
		done: make(chan struct{}),
	}
	h.add(c)

	// writePump: one goroutine per client — gorilla/websocket requires
	// that writes happen from a single goroutine per connection.
	go func() {
		defer func() {
			h.remove(c)
			conn.Close()
		}()
		for {
			select {
			case msg := <-c.send:
				conn.SetWriteDeadline(time.Now().Add(writeTimeout))
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			case <-c.done:
				return
			}
		}
	}()

	return c
}

func (h *Hub) Unregister(c *client) {
	h.remove(c)
}

func (h *Hub) Send(c *client, msg []byte) bool {
	select {
	case <-c.done:
		return false
	case c.send <- msg:
		return true
	default:
		log.Printf("[hub] client buffer full, disconnecting")
		h.remove(c)
		return false
	}
}

func (h *Hub) broadcast(msg []byte) {
	h.mu.RLock()
	clients := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		h.Send(c, msg)
	}
}

// Run reads from alertChan, logs every alert, and broadcasts to all clients.
func (h *Hub) Run(alertChan <-chan types.Alert) {
	for a := range alertChan {
		dir := "▲"
		if a.Direction == "down" {
			dir = "▼"
		}
		log.Printf("[%s] [%s] %s %s cur=%.2f%% max=%.2f%% за %dms  %.6f → %.6f",
			a.Timestamp.Format("15:04:05"),
			a.ImpulseID,
			a.Symbol, dir,
			a.CurrentPct,
			a.MaxPtc,
			a.DurationMs,
			a.StartPrice, a.EndPrice,
		)

		msg, err := json.Marshal(a)
		if err != nil {
			continue
		}
		h.broadcast(msg)
	}
}
