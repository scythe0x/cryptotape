package desktop

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
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
