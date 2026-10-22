package licenseserver

import (
	"crypto/ed25519"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"cryptotape/internal/licensing"
)

type quota struct {
	at    time.Time
	count int
}

func Handler(store *Store, signingKey ed25519.PrivateKey) http.Handler {
	mux := http.NewServeMux()
	var mu sync.Mutex
	clients := make(map[string]quota)
	for _, route := range []string{"activate", "validate"} {
		mux.HandleFunc("POST /v1/"+route, func(w http.ResponseWriter, r *http.Request) {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				http.Error(w, "invalid peer", 400)
				return
			}
			// caddy is the sole local proxy and replaces this header with its peer address.
			if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
				if forwarded := r.Header.Get("X-Real-IP"); net.ParseIP(forwarded) != nil {
					host = forwarded
				}
			}
			now := time.Now()
			mu.Lock()
			if len(clients) > 10000 {
				for k, v := range clients {
					if now.Sub(v.at) > time.Minute {
						delete(clients, k)
					}
				}
			}
			q := clients[host]
			if now.Sub(q.at) >= time.Minute {
				q = quota{at: now}
			}
			q.count++
			allowed := q.count <= 30 && (len(clients) < 10001 || q.count > 1)
			if allowed {
				clients[host] = q
			}
			mu.Unlock()
			if !allowed {
				http.Error(w, "too many requests", 429)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 4096)
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			var request licensing.Request
			if err := decoder.Decode(&request); err != nil || !licensing.ValidRequest(request) {
				http.Error(w, "invalid request", 400)
				return
			}
			if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
				http.Error(w, "invalid request", 400)
				return
			}
			grant, err := store.Check(r.Context(), request, route == "activate", now)
			if err != nil {
				slog.Error("license storage failed")
				http.Error(w, "unavailable", 503)
				return
			}
			signed, err := licensing.Sign(grant, signingKey)
			if err != nil {
				slog.Error("license signing failed")
				http.Error(w, "unavailable", 503)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			if err := json.NewEncoder(w).Encode(signed); err != nil {
				slog.Debug("license response interrupted")
			}
		})
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := store.db.PingContext(r.Context()); err != nil {
			http.Error(w, "unavailable", 503)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}
