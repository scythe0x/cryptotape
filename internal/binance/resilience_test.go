package binance

import (
	"context"
	"encoding/json"
	"errors"
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

func localFeed(t *testing.T, handler http.Handler, callbacks Callbacks) *Feed {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	t.Cleanup(transport.CloseIdleConnections)
	return &Feed{client: &http.Client{Transport: streamTransport{target: endpoint, base: transport}}, cb: callbacks,
		shardMembers: [][]string{{"BTCUSDT"}}, updates: []chan struct{}{make(chan struct{}, 1)},
		known: map[string]bool{"BTCUSDT": true}, eligible: map[string]bool{"BTCUSDT": true}, volumes: map[string]float64{"BTCUSDT": 100e6}, symbolShard: map[string]int{"BTCUSDT": 0}, shards: 1,
	}
}

func TestRateLimitGateBlocksAllRESTServices(t *testing.T) {
	var calls atomic.Int32
	f := localFeed(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "600")
		w.Header().Set("X-MBX-USED-WEIGHT-1M", "2400")
		w.WriteHeader(429)
		fmt.Fprint(w, `{"code":-1003}`)
	}), Callbacks{})
	var result struct{}
	err := f.get(context.Background(), "/fapi/v1/time", &result)
	var limited *RateLimitError
	if !errors.As(err, &limited) || limited.Status != 429 || f.usedWeight != 2400 {
		t.Fatalf("rate limit not recorded: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := f.get(ctx, "/fapi/v1/exchangeInfo", &result); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("gate ignored: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("sent %d requests during ban", calls.Load())
	}
}

func TestBanDeadlineParsing(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	until := now.Add(time.Hour)
	body := []byte(fmt.Sprintf(`{"msg":"IP banned until %d. Please use websocket"}`, until.UnixMilli()))
	if got := retryDeadline("", body, 418, now); !got.Equal(until) {
		t.Fatalf("ban expiration lost: %s", got)
	}
	if got := retryDeadline("120", nil, 429, now); !got.Equal(now.Add(2 * time.Minute)) {
		t.Fatal(got)
	}
	if got := retryDeadline(until.UTC().Format(http.TimeFormat), nil, 429, now); !got.Equal(until) {
		t.Fatal(got)
	}
	if got := retryDeadline("", nil, 418, now); got.Before(now.Add(72 * time.Hour)) {
		t.Fatal("unsafe fallback")
	}
}

func TestRefreshFailurePreservesPricesAndUpdatesOtherServices(t *testing.T) {
	volumes := make(chan map[string]float64, 1)
	f := localFeed(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/fapi/v1/time":
			http.Error(w, "temporary", 503)
		case "/fapi/v1/exchangeInfo":
			fmt.Fprint(w, `{"symbols":[{"symbol":"BTCUSDT","status":"TRADING","quoteAsset":"USDT"}]}`)
		case "/fapi/v1/ticker/24hr":
			fmt.Fprint(w, `[{"symbol":"BTCUSDT","quoteVolume":"90000000"}]`)
		}
	}), Callbacks{Volume: func(v map[string]float64) { volumes <- v }, Gap: func([]string) { t.Error("service failure reset price history") }})
	for name, refresh := range f.refreshServices() {
		err := refresh(context.Background())
		if (name == "clock") != (err != nil) {
			t.Fatalf("service %s: %v", name, err)
		}
	}
	if len(volumes) != 1 || !f.eligible["BTCUSDT"] {
		t.Fatal("one service blocked the others")
	}
}

func TestSubscriptionErrorsAndTimeout(t *testing.T) {
	for _, mode := range []string{"error", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			f := localFeed(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, nil)
				if err != nil {
					return
				}
				defer conn.CloseNow()
				_, _, err = conn.Read(r.Context())
				if err != nil {
					return
				}
				if mode == "error" {
					_ = websocketWriteJSON(r.Context(), conn, map[string]any{"id": 1, "code": 2, "msg": "invalid request"})
				}
				_, _, _ = conn.Read(r.Context())
			}), Callbacks{Health: func(core.Health) {}, Gap: func([]string) {}, Quote: func(core.Quote) { t.Error("unconfirmed subscription delivered a quote") }})
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			defer cancel()
			_, err := f.readBook(ctx, 0)
			if err == nil || !strings.Contains(err.Error(), "subscription") {
				t.Fatalf("ack failure hidden: %v", err)
			}
			if f.streams[0].health.Subscriptions != 0 {
				t.Fatal("unconfirmed subscriptions counted")
			}
		})
	}
}

func TestRotationDoesNotResetOrDuplicateSignals(t *testing.T) {
	var connections, gaps, signals atomic.Int32
	engine := core.NewEngine([]core.Rule{{ID: 1, Percent: 5, WindowSeconds: 300}}, core.DefaultSettings())
	at := time.Now().UnixMilli()
	f := localFeed(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		_, _, err = conn.Read(r.Context())
		if err != nil {
			return
		}
		_ = websocketWriteJSON(r.Context(), conn, map[string]any{"id": 1, "result": nil})
		for i, price := range []int{100, 94} {
			raw := fmt.Sprintf(`{"data":{"s":"BTCUSDT","b":"%d","B":"7000","a":"%d","A":"8000","E":%d,"u":%d}}`, price, price, at+int64(i), i+1)
			if err := conn.Write(r.Context(), websocket.MessageText, []byte(raw)); err != nil {
				return
			}
		}
		connections.Add(1)
		_, _, _ = conn.Read(r.Context())
	}), Callbacks{Health: func(core.Health) {}, Gap: func(symbols []string) { gaps.Add(1); engine.Reset(symbols) }, Quote: func(q core.Quote) { signals.Add(int32(len(engine.Process(q)))) }})
	f.rotationAfter = 30 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- f.bookLoop(ctx, 0) }()
	deadline := time.After(800 * time.Millisecond)
	for connections.Load() < 3 {
		select {
		case <-deadline:
			t.Fatal("rotation never completed")
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("rotation workers leaked")
	}
	if gaps.Load() != 1 || signals.Load() != 1 {
		t.Fatalf("rotation reset/doubled history: gaps=%d signals=%d", gaps.Load(), signals.Load())
	}
}

func TestListingGrowthAddsBoundedShards(t *testing.T) {
	f := &Feed{known: map[string]bool{}, volumes: map[string]float64{}, eligible: map[string]bool{}, cb: Callbacks{Gap: func([]string) {}}}
	var symbols []string
	for i := range 1100 {
		symbols = append(symbols, fmt.Sprintf("COIN%dUSDT", i))
	}
	f.refreshSymbols(symbols)
	if f.shards != 6 {
		t.Fatalf("shards=%d", f.shards)
	}
	for _, members := range f.shardMembers {
		if len(members) > 200 {
			t.Fatal("shard overflow")
		}
	}
}

func TestRawEventClockAndMissingTimestamp(t *testing.T) {
	for _, mode := range []string{"offset", "trade fallback", "missing"} {
		t.Run(mode, func(t *testing.T) {
			received := make(chan core.Quote, 1)
			event := time.Now().Add(time.Hour).UnixMilli()
			f := localFeed(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, nil)
				if err != nil {
					return
				}
				defer conn.CloseNow()
				if _, _, err := conn.Read(r.Context()); err != nil {
					return
				}
				_ = websocketWriteJSON(r.Context(), conn, map[string]any{"id": 1, "result": nil})
				eventField, tradeField := event, int64(0)
				if mode == "trade fallback" {
					eventField, tradeField = 0, event
				}
				if mode == "missing" {
					eventField, tradeField = 0, 0
				}
				raw := fmt.Sprintf(`{"data":{"e":"bookTicker","s":"BTCUSDT","b":"100","B":"7000","a":"102","A":"8000","E":%d,"T":%d,"u":99}}`, eventField, tradeField)
				_ = conn.Write(r.Context(), websocket.MessageText, []byte(raw))
				_ = conn.Close(websocket.StatusNormalClosure, "complete")
			}), Callbacks{Health: func(core.Health) {}, Quote: func(q core.Quote) { received <- q }})
			f.clockOffset.Store(int64(time.Hour))
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, _ = f.readBook(ctx, 0)
			if mode == "missing" {
				if len(received) != 0 {
					t.Fatal("receipt time invented an exchange timestamp")
				}
				return
			}
			select {
			case q := <-received:
				if q.EventTime.UnixMilli() != event || q.UpdateID != 99 || q.ClockOffset != time.Hour || q.Bid != 100 || q.Ask != 102 {
					t.Fatalf("incorrect raw quote: %+v", q)
				}
			default:
				t.Fatal("timestamped quote lost")
			}
		})
	}
}

func TestAgeHistogramExpiresSamples(t *testing.T) {
	var histogram ageHistogram
	now := time.Now()
	histogram.add(now, 5850*time.Millisecond)
	p99, maximum := histogram.snapshot(now)
	if p99 != 10000 || maximum != 5850 {
		t.Fatalf("p99=%d max=%d", p99, maximum)
	}
	p99, maximum = histogram.snapshot(now.Add(31 * time.Second))
	if p99 != 0 || maximum != 0 {
		t.Fatal("expired latency retained")
	}
}

func TestRouteBanSurvivesNewFeedAndCredentialChanges(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "600")
		w.WriteHeader(418)
	}))
	defer server.Close()
	first, err := NewFeed("http://user:example@ban-fixture.invalid:443", 0, Callbacks{})
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	defer transport.CloseIdleConnections()
	first.client.Transport = streamTransport{target: endpoint, base: transport}
	var out struct{}
	if err := first.get(context.Background(), "/fapi/v1/time", &out); err == nil {
		t.Fatal("ban not recorded")
	}
	second, err := NewFeed("http://another:example@ban-fixture.invalid:443", 0, Callbacks{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := second.get(ctx, "/fapi/v1/time", &out); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("replacement bypassed gate: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatal("sent another request during ban")
	}
}

func TestRotationFailureLeavesOldPriceStreamRunning(t *testing.T) {
	var attempts, quotes, gaps atomic.Int32
	f := localFeed(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) > 1 {
			http.Error(w, "replacement unavailable", 503)
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		if _, _, err := conn.Read(r.Context()); err != nil {
			return
		}
		_ = websocketWriteJSON(r.Context(), conn, map[string]any{"id": 1, "result": nil})
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		go func() { _, _, _ = conn.Read(ctx); cancel() }()
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				raw := fmt.Sprintf(`{"data":{"s":"BTCUSDT","b":"100","a":"101","E":%d}}`, time.Now().UnixMilli())
				if err := conn.Write(ctx, websocket.MessageText, []byte(raw)); err != nil {
					return
				}
			}
		}
	}), Callbacks{Health: func(core.Health) {}, Gap: func([]string) { gaps.Add(1) }, Quote: func(core.Quote) { quotes.Add(1) }})
	f.rotationAfter = 20 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- f.bookLoop(ctx, 0) }()
	deadline := time.After(500 * time.Millisecond)
	for attempts.Load() < 2 {
		select {
		case <-deadline:
			t.Fatal("replacement not attempted")
		case <-time.After(5 * time.Millisecond):
		}
	}
	before := quotes.Load()
	timer := time.NewTimer(50 * time.Millisecond)
	defer timer.Stop()
	<-timer.C
	if quotes.Load() <= before || gaps.Load() != 1 {
		t.Fatalf("replacement failure interrupted prices: quotes=%d before=%d gaps=%d", quotes.Load(), before, gaps.Load())
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("rotation workers did not stop")
	}
}

func TestSubscriptionWritesArePaced(t *testing.T) {
	arrivals := make(chan time.Time, 6)
	f := localFeed(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		for range 6 {
			_, raw, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			var request struct {
				ID int64 `json:"id"`
			}
			if err := json.Unmarshal(raw, &request); err != nil {
				return
			}
			arrivals <- time.Now()
			if err := websocketWriteJSON(r.Context(), conn, map[string]any{"id": request.ID, "result": nil}); err != nil {
				return
			}
		}
		_, _, _ = conn.Read(r.Context())
	}), Callbacks{})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsPublic, &websocket.DialOptions{HTTPClient: f.client})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	control := newSubscriptions(conn)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			_, raw, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var frame bookFrame
			if json.Unmarshal(raw, &frame) == nil && frame.ID != nil {
				control.ack(ctx, *frame.ID, frame)
			}
		}
	}()
	for range 6 {
		if err := control.send(ctx, "SUBSCRIBE", []string{"btcusdt@bookTicker"}, nil); err != nil {
			t.Fatal(err)
		}
	}
	first, last := <-arrivals, time.Time{}
	for range 5 {
		last = <-arrivals
	}
	if last.Sub(first) < 990*time.Millisecond {
		t.Fatalf("control limiter bypassed: %s", last.Sub(first))
	}
	cancel()
	conn.CloseNow()
	<-readDone
}

func TestTurnoverBeforeContractRefreshDoesNotDelayNewListing(t *testing.T) {
	f := &Feed{minVolume: 80e6, known: map[string]bool{"AAAUSDT": true}, volumes: map[string]float64{"AAAUSDT": 90e6}, eligible: map[string]bool{"AAAUSDT": true}, shardMembers: [][]string{{"AAAUSDT"}}, symbolShard: map[string]int{"AAAUSDT": 0}, updates: []chan struct{}{make(chan struct{}, 1)}, cb: Callbacks{Gap: func([]string) {}}}
	f.updateEligibility(map[string]float64{"AAAUSDT": 90e6, "NEWUSDT": 100e6})
	f.refreshSymbols([]string{"AAAUSDT", "NEWUSDT"})
	if !f.eligible["NEWUSDT"] || len(f.desiredStreams(0)) != 2 {
		t.Fatal("new listing waits another five minutes for an already available turnover snapshot")
	}
}
