package desktop

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func tigerPort(t *testing.T, endpoint string) int {
	t.Helper()
	parsed, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatal(err)
	}
	return port
}

func TestTigerMultiplexesEveryRequestAndCorrelatesReplies(t *testing.T) {
	var received, connections atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		connections.Add(1)
		defer connection.CloseNow()
		for {
			_, raw, err := connection.Read(r.Context())
			if err != nil {
				return
			}
			var request struct {
				Type, RequestID, Symbol string
				Exchange                string `json:"e"`
				Market                  string `json:"m"`
				Group                   string `json:"linkGroup"`
			}
			if err = json.Unmarshal(raw, &request); err != nil {
				t.Error(err)
				return
			}
			if request.Type != "SETLINKSYMBOL" || request.Exchange != "BINANCE-FUT" || request.Market != "FUTURES" || request.Group != "A" {
				t.Errorf("wrong Tiger command: %s", raw)
			}
			received.Add(1)
			kind := "successfully"
			if request.Symbol == "ERRORUSDT" {
				kind = "error"
			}
			reply := map[string]any{"requestId": request.RequestID, "response": map[string]any{"type": kind, "error": map[string]any{"message": "symbol not found", "code": 400}}}
			data, err := json.Marshal(reply)
			if err != nil {
				t.Error(err)
				return
			}
			if err = connection.Write(r.Context(), websocket.MessageText, data); err != nil {
				return
			}
		}
	}))
	defer server.Close()
	tiger := NewTiger()
	defer tiger.Close()
	port := tigerPort(t, server.URL)
	var wg sync.WaitGroup
	for index := range 750 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := tiger.Open(ctx, "COIN"+strconv.Itoa(index)+"USDT", port, "A"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if received.Load() != 750 || connections.Load() != 1 {
		t.Fatalf("requests=%d connections=%d", received.Load(), connections.Load())
	}
	if err := tiger.Open(context.Background(), "ERRORUSDT", port, "A"); err == nil {
		t.Fatal("Tiger rejection was hidden")
	}
}

func TestTigerTimeoutDoesNotLeavePendingRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer connection.CloseNow()
		for {
			if _, _, err := connection.Read(r.Context()); err != nil {
				return
			}
		}
	}))
	defer server.Close()
	tiger := NewTiger()
	defer tiger.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := tiger.Open(ctx, "BTCUSDT", tigerPort(t, server.URL), "A"); err == nil {
		t.Fatal("missing response reported success")
	}
	tiger.mu.Lock()
	defer tiger.mu.Unlock()
	if len(tiger.pending) != 0 {
		t.Fatal("expired request was retained")
	}
}

func TestTigerReconnectsAfterTerminalClosesConnection(t *testing.T) {
	var connections atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer connection.CloseNow()
		connections.Add(1)
		_, raw, err := connection.Read(r.Context())
		if err != nil {
			return
		}
		var request struct{ RequestID string }
		if err = json.Unmarshal(raw, &request); err != nil {
			t.Error(err)
			return
		}
		data, err := json.Marshal(map[string]any{"requestId": request.RequestID, "response": map[string]string{"type": "successfully"}})
		if err != nil {
			t.Error(err)
			return
		}
		if err = connection.Write(r.Context(), websocket.MessageText, data); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	tiger := NewTiger()
	defer tiger.Close()
	for range 2 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := tiger.Open(ctx, "BTCUSDT", tigerPort(t, server.URL), "A")
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(time.Second)
		for {
			tiger.mu.Lock()
			closed := tiger.conn == nil
			tiger.mu.Unlock()
			if closed {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("closed socket retained")
			}
			time.Sleep(time.Millisecond)
		}
	}
	if connections.Load() != 2 {
		t.Fatal("did not reconnect")
	}
}
