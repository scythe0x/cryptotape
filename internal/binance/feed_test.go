package binance

import (
	"context"
	"encoding/json"
	"github.com/coder/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBookTickerKeepsPricesSeparateFromQuantities(t *testing.T) {
	var tick bookTick
	message := `{"e":"bookTicker","s":"BTCUSDT","b":"82057.70","B":"6.294","a":"82057.80","A":"1.611","T":1792468216127,"E":1792468216127}`
	if err := json.Unmarshal([]byte(message), &tick); err != nil {
		t.Fatal(err)
	}
	if tick.Bid != "82057.70" || tick.Ask != "82057.80" || tick.BidQty != "6.294" || tick.AskQty != "1.611" || tick.Event != 1792468216127 {
		t.Fatalf("incorrect book ticker decode: %+v", tick)
	}
}

func TestProxyFailureDoesNotUseDirectRoute(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	feed, err := NewFeed(strings.TrimPrefix(server.URL, "http://"), 0, Callbacks{})
	if err != nil {
		t.Fatal(err)
	}
	if err := feed.Test(context.Background()); err == nil {
		t.Fatal("Binance test unexpectedly succeeded through a failing proxy")
	}
	if requests.Load() == 0 {
		t.Fatal("proxy was bypassed")
	}
}

func TestEligibilityChangesUseCurrentTurnover(t *testing.T) {
	feed := &Feed{
		minVolume:    80_000_000,
		symbols:      []string{"AAAUSDT", "BBBUSDT"},
		known:        map[string]bool{"AAAUSDT": true, "BBBUSDT": true},
		volumes:      map[string]float64{"AAAUSDT": 90_000_000},
		eligible:     map[string]bool{"AAAUSDT": true},
		shardMembers: [][]string{{"AAAUSDT", "BBBUSDT"}},
		updates:      []chan struct{}{make(chan struct{}, 1)},
	}
	feed.updateEligibility(map[string]float64{"AAAUSDT": 79_000_000, "BBBUSDT": 81_000_000})
	streams := feed.desiredStreams(0)
	if len(streams) != 1 || streams[0] != "bbbusdt@bookTicker" {
		t.Fatalf("wanted BBB only after threshold change, got %v", streams)
	}
	select {
	case <-feed.updates[0]:
	default:
		t.Fatal("book stream was not told about changed eligibility")
	}
}

func TestPingReportsRoundTripAndStopsOnCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer connection.CloseNow()
		connection.Read(r.Context())
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	connection, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.CloseNow()
	go connection.Read(ctx)
	reports := make(chan time.Duration, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		pingUntilDone(ctx, connection, func(elapsed time.Duration) { reports <- elapsed })
	}()
	select {
	case elapsed := <-reports:
		if elapsed <= 0 {
			t.Fatalf("invalid RTT: %s", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no initial ping measurement")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ping loop did not stop")
	}
}

func TestListingRefreshPreservesExistingShardsAndAddsNewMarket(t *testing.T) {
	feed := &Feed{minVolume: 80e6, known: map[string]bool{"AAAUSDT": true, "BBBUSDT": true}, volumes: map[string]float64{"AAAUSDT": 90e6, "BBBUSDT": 90e6}, eligible: map[string]bool{"AAAUSDT": true, "BBBUSDT": true}, shardMembers: [][]string{{"AAAUSDT"}, {"BBBUSDT"}}, updates: []chan struct{}{make(chan struct{}, 1), make(chan struct{}, 1)}, cb: Callbacks{Gap: func([]string) {}}}
	feed.refreshSymbols([]string{"AAAUSDT", "BBBUSDT", "NEWUSDT"})
	feed.updateEligibility(map[string]float64{"NEWUSDT": 100e6})
	if feed.symbolShard["AAAUSDT"] != 0 || feed.symbolShard["BBBUSDT"] != 1 || !feed.eligible["NEWUSDT"] {
		t.Fatal("listing reset existing shards or was missed")
	}
	feed.SetMinimumVolume(120e6)
	if len(feed.desiredStreams(0))+len(feed.desiredStreams(1)) != 0 {
		t.Fatal("subscription floor not applied")
	}
	feed.SetMinimumVolume(85e6)
	if len(feed.desiredStreams(0))+len(feed.desiredStreams(1)) != 3 {
		t.Fatal("eligible markets not subscribed")
	}
}
