package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type pingMessage struct {
	Type     string `json:"type"`
	ClientTs int64  `json:"clientTs"`
}

type pongMessage struct {
	Type     string `json:"type"`
	ClientTs int64  `json:"clientTs"`
	ServerTs int64  `json:"serverTs"`
}

func StartServer(addr string, hub *Hub) {
	mux := http.NewServeMux()

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("[server] upgrade error: %v", err)
			return
		}
		client := hub.Register(conn)
		defer hub.Unregister(client)

		log.Printf("[server] client connected: %s", r.RemoteAddr)

		// readPump: drain incoming frames (ping/pong/close) so the
		// connection stays healthy. Actual data flows server→client only.
		for {
			msgType, payload, err := conn.ReadMessage()
			if err != nil {
				log.Printf("[server] client disconnected: %s", r.RemoteAddr)
				return
			}

			if msgType != websocket.TextMessage {
				continue
			}

			var ping pingMessage
			if err := json.Unmarshal(payload, &ping); err != nil || ping.Type != "ping" {
				continue
			}

			pong, err := json.Marshal(pongMessage{
				Type:     "pong",
				ClientTs: ping.ClientTs,
				ServerTs: time.Now().UnixMilli(),
			})
			if err != nil {
				continue
			}

			if ok := hub.Send(client, pong); !ok {
				return
			}
		}
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Printf("[server] listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("[server] fatal: %v", err)
	}
}
