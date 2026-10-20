package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"cryptotape/internal/core"
	"github.com/coder/websocket"
)

// keep production stream URLs while routing requests to a local websocket fixture.
type streamTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (s streamTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	redirected := request.Clone(request.Context())
	endpoint := *request.URL
	endpoint.Scheme, endpoint.Host = s.target.Scheme, s.target.Host
	redirected.URL = &endpoint
	redirected.Host = endpoint.Host
	return s.base.RoundTrip(redirected)
}

func TestBookStreamDecodesAndFiltersRealFrames(t *testing.T) {
	for _, test := range []struct {
		name                  string
		eligible, volumeReady bool
		age                   time.Duration
		wantQuote, wantStale  bool
	}{
		{"fresh", true, true, 0, true, false},
		{"stale", true, true, 11 * time.Second, false, true},
		{"future", true, true, -3 * time.Second, false, true},
		{"below turnover", false, true, 0, false, false},
		{"volume disconnected", true, false, 0, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				connection, err := websocket.Accept(w, r, nil)
				if err != nil {
					t.Error(err)
					return
				}
				defer connection.CloseNow()
				_, raw, err := connection.Read(r.Context())
				if err != nil {
					t.Error(err)
					return
				}
				var subscription struct {
					Method string
					Params []string
				}
				if err := json.Unmarshal(raw, &subscription); err != nil ||
					subscription.Method != "SUBSCRIBE" || len(subscription.Params) != 1 || subscription.Params[0] != "btcusdt@bookTicker" {
					t.Errorf("incorrect book subscription: %s %v", raw, err)
					return
				}
				frame := fmt.Sprintf(`{"stream":"btcusdt@bookTicker","data":{"e":"bookTicker","s":"BTCUSDT","b":"100","B":"7000","a":"102","A":"8000","E":%d}}`,
					time.Now().Add(-test.age).UnixMilli())
				if err := connection.Write(r.Context(), websocket.MessageText, []byte(frame)); err != nil {
					t.Error(err)
					return
				}
				if err := connection.Close(websocket.StatusNormalClosure, "fixture complete"); err != nil {
					return
				}
			}))
			defer server.Close()
			endpoint, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			quotes := make(chan core.Quote, 1)
			transport := http.DefaultTransport.(*http.Transport).Clone()
			defer transport.CloseIdleConnections()
			feed := &Feed{
				client:       &http.Client{Transport: streamTransport{target: endpoint, base: transport}},
				shardMembers: [][]string{{"BTCUSDT"}}, eligible: map[string]bool{"BTCUSDT": true},
				updates: []chan struct{}{make(chan struct{}, 1)}, shards: 1,
				cb: Callbacks{Health: func(core.Health) {}, Quote: func(q core.Quote) { quotes <- q }},
			}
			feed.volumeOK.Store(test.volumeReady)
			// eligibility is removed after subscription, as during a turnover refresh.
			if !test.eligible {
				feed.cb.Health = func(core.Health) {
					feed.mu.Lock()
					feed.eligible["BTCUSDT"] = false
					feed.mu.Unlock()
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			err = feed.readBook(ctx, 0)
			if test.wantStale && (err == nil || !strings.Contains(err.Error(), "stale book stream")) {
				t.Fatalf("unsafe event was not rejected: %v", err)
			}
			if (len(quotes) > 0) != test.wantQuote {
				t.Fatalf("received %d quotes, expected quote=%v, error=%v", len(quotes), test.wantQuote, err)
			}
			if test.wantQuote {
				q := <-quotes
				if q.Symbol != "BTCUSDT" || q.Bid != 100 || q.Ask != 102 || q.Received.Sub(q.EventTime) > time.Second {
					t.Fatalf("incorrect quote delivered to detector: %+v", q)
				}
			}
		})
	}
}

func TestFeedStartupAndReconnectProduceFreshSignals(t *testing.T) {
	settings := core.DefaultSettings()
	settings.CooldownSeconds = 0
	engine := core.NewEngine([]core.Rule{{ID: 1, Percent: 5, WindowSeconds: 300, MinQuoteVolume: 80e6}}, settings)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var connections atomic.Int32
	signals := make(chan core.Candidate, 10)
	received := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/fapi/v1/time":
			fmt.Fprintf(w, `{"serverTime":%d}`, time.Now().UnixMilli())
		case "/fapi/v1/exchangeInfo":
			fmt.Fprint(w, `{"symbols":[{"symbol":"BTCUSDT","status":"TRADING","quoteAsset":"USDT"},{"symbol":"LOWUSDT","status":"TRADING","quoteAsset":"USDT"},{"symbol":"OLDUSDT","status":"SETTLING","quoteAsset":"USDT"}]}`)
		case "/fapi/v1/ticker/24hr":
			fmt.Fprint(w, `[{"symbol":"BTCUSDT","quoteVolume":"100000000"},{"symbol":"LOWUSDT","quoteVolume":"10000000"}]`)
		case "/market/stream", "/public/stream":
			connection, err := websocket.Accept(w, r, nil)
			if err != nil {
				t.Error(err)
				return
			}
			defer connection.CloseNow()
			if r.URL.Path == "/market/stream" {
				frame := fmt.Sprintf(`{"data":[{"e":"24hrMiniTicker","s":"BTCUSDT","q":"100000000","E":%d}]}`, time.Now().UnixMilli())
				if err := connection.Write(r.Context(), websocket.MessageText, []byte(frame)); err != nil {
					t.Error(err)
					return
				}
				connection.Read(r.Context())
				return
			}
			var subscription struct {
				Method string
				Params []string
			}
			_, raw, err := connection.Read(r.Context())
			if err != nil {
				t.Error(err)
				return
			}
			if err := json.Unmarshal(raw, &subscription); err != nil || len(subscription.Params) != 1 ||
				subscription.Params[0] != "btcusdt@bookTicker" {
				t.Errorf("incorrect eligible subscription: %s %v", raw, err)
				return
			}
			prices := []float64{100, 94}
			attempt := connections.Add(1)
			if attempt > 1 {
				prices = []float64{50, 47}
			}
			for _, price := range prices {
				frame := fmt.Sprintf(`{"data":{"s":"BTCUSDT","b":"%f","a":"%f","E":%d}}`, price, price, time.Now().UnixMilli())
				if err := connection.Write(r.Context(), websocket.MessageText, []byte(frame)); err != nil {
					t.Error(err)
					return
				}
			}
			if attempt == 1 {
				connection.Close(websocket.StatusNormalClosure, "simulate disconnection")
			} else {
				connection.Read(r.Context())
			}
		default:
			t.Errorf("unexpected Binance endpoint: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	feed, err := NewFeed("", 80e6, Callbacks{
		Volume: engine.SetVolumes,
		Gap: func(symbols []string) {
			if symbols == nil {
				engine.ResetAll()
			} else {
				engine.Reset(symbols)
			}
		},
		Health: func(core.Health) {},
		Quote: func(q core.Quote) {
			for _, candidate := range engine.Process(q) {
				signals <- candidate
			}
			received++
			if received == 4 {
				cancel()
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	defer transport.CloseIdleConnections()
	feed.client.Transport = streamTransport{target: endpoint, base: transport}
	err = feed.Run(ctx)
	if err != context.Canceled || received != 4 || connections.Load() != 2 {
		t.Fatalf("feed failed to start/reconnect: error=%v quotes=%d connections=%d", err, received, connections.Load())
	}
	if len(signals) != 2 {
		t.Fatalf("missing signals or prices bridged a gap: %d signals", len(signals))
	}
	for range 2 {
		candidate := <-signals
		if candidate.Direction != "down" || candidate.ChangePercent < -6.01 || candidate.ChangePercent > -5.99 {
			t.Fatalf("signal used stale prices: %+v", candidate)
		}
	}
}
