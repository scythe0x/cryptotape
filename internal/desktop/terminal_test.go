package desktop

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestOpenTerminalUsesExactFuturesRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/open/zenica/binance/F/BTCUSDT" {
			t.Errorf("unexpected terminal request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","message":"Vataga triggered successfully."}`))
	}))
	defer server.Close()
	terminal := NewTerminal()
	terminal.baseURL = server.URL
	if err := terminal.Open(context.Background(), "BTCUSDT"); err != nil {
		t.Fatal(err)
	}
}

func TestOpenTerminalRejectsInvalidSymbol(t *testing.T) {
	terminal := NewTerminal()
	if err := terminal.Open(context.Background(), "BTC/USDT"); err == nil {
		t.Fatal("invalid symbol was accepted")
	}
}

func TestOpenTerminalReportsFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	terminal := NewTerminal()
	terminal.baseURL = server.URL
	if err := terminal.Open(context.Background(), "BTCUSDT"); err == nil {
		t.Fatal("terminal failure was hidden")
	}
}

func TestMarketCrashOpensEveryMarket(t *testing.T) {
	const markets = 750
	var received atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		received.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()
	terminal := NewTerminal()
	terminal.baseURL = server.URL
	var wg sync.WaitGroup
	errors := make(chan error, markets)
	latencies := make(chan time.Duration, markets)
	started := time.Now()
	for index := range markets {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			start := time.Now()
			if err := terminal.Open(context.Background(), fmt.Sprintf("COIN%dUSDT", index)); err != nil {
				errors <- err
			}
			latencies <- time.Since(start)
		}(index)
	}
	wg.Wait()
	close(errors)
	close(latencies)
	if len(errors) > 0 {
		t.Fatalf("%d terminal requests failed; first: %v", len(errors), <-errors)
	}
	if received.Load() != markets {
		t.Fatalf("only %d/%d terminal commands arrived", received.Load(), markets)
	}
	measurements := make([]time.Duration, 0, markets)
	for latency := range latencies {
		measurements = append(measurements, latency)
	}
	slices.Sort(measurements)
	t.Logf("%d terminal commands: total=%s request p99=%s", markets, time.Since(started), measurements[markets*99/100])
}
