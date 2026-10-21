package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"cryptotape/internal/core"
	"github.com/coder/websocket"
)

func TestVatagaExchangePriorityStopsAtFirstSuccess(t *testing.T) {
	var requests []string
	var requestMu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMu.Lock()
		requests = append(requests, r.URL.Path)
		requestMu.Unlock()
		if strings.Contains(r.URL.Path, "/bybit/") {
			fmt.Fprint(w, `{"status":"ok"}`)
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	terminal := NewTerminal()
	terminal.baseURL = server.URL
	c := &Controller{terminal: terminal}
	if err := c.openInTerminal(context.Background(), "REZUSDT", "vataga", "okx", "binance", "bybit", "gate"); err != nil {
		t.Fatal(err)
	}
	requestMu.Lock()
	defer requestMu.Unlock()
	expected := []string{"/api/open/zenica/okx/F/REZ-USDT-SWAP", "/api/open/zenica/binance/F/REZUSDT", "/api/open/zenica/bybit/F/REZUSDT"}
	if !reflect.DeepEqual(requests, expected) {
		t.Fatalf("priority ignored: %v", requests)
	}
}

func TestVatagaPriorityDoesNotHideServerFailures(t *testing.T) {
	var requests int
	var countMu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		countMu.Lock()
		requests++
		countMu.Unlock()
		w.WriteHeader(500)
	}))
	defer server.Close()
	terminal := NewTerminal()
	terminal.baseURL = server.URL
	c := &Controller{terminal: terminal}
	err := c.openInTerminal(context.Background(), "REZUSDT", "vataga", "binance", "bybit")
	countMu.Lock()
	defer countMu.Unlock()
	if err == nil || requests != 1 {
		t.Fatalf("server failure hidden: %d %v", requests, err)
	}
}

func TestManualListingOpenUsesSavedExchangePriority(t *testing.T) {
	c, _ := testController(t, func(string, any) {})
	var requests []string
	var requestMu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMu.Lock()
		requests = append(requests, r.URL.Path)
		requestMu.Unlock()
		if strings.Contains(r.URL.Path, "/gate/") {
			fmt.Fprint(w, `{"status":"ok"}`)
		} else {
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	c.terminal.baseURL = server.URL
	rule, err := c.SaveRule(core.Rule{Kind: core.Listing, Name: "listing", Sound: "none", Color: "#ffffff", ListingTypes: []string{"spot"}, OpenExchanges: []string{"okx", "gate", "bybit", "binance"}})
	if err != nil {
		t.Fatal(err)
	}
	alert, err := c.store.InsertAlert(core.Candidate{Rule: rule, Symbol: "REZUSDT", Listing: &core.ListingDetails{Asset: "REZ", Exchange: "upbit", MarketType: "spot"}})
	if err != nil {
		t.Fatal(err)
	}
	result := c.OpenTerminals(alert.ID, alert.Symbol, []string{"vataga"})
	if result.Opened != 1 || result.Error != "" {
		t.Fatalf("manual open: %+v", result)
	}
	requestMu.Lock()
	defer requestMu.Unlock()
	if len(requests) != 2 || !strings.Contains(requests[0], "/okx/") || !strings.Contains(requests[1], "/gate/") {
		t.Fatalf("manual priority: %v", requests)
	}
}

func TestTigerPriorityUsesEchoedRequestAndSymbolRejection(t *testing.T) {
	var exchanges []string
	var exchangeMu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer connection.CloseNow()
		for {
			_, raw, err := connection.Read(r.Context())
			if err != nil {
				return
			}
			var request map[string]any
			if err := json.Unmarshal(raw, &request); err != nil {
				t.Error(err)
				return
			}
			exchange, _ := request["e"].(string)
			exchangeMu.Lock()
			exchanges = append(exchanges, exchange)
			exchangeMu.Unlock()
			reply := map[string]any{"request": request, "response": map[string]any{"type": "successfully"}}
			// the documented protocol echoes the request and need not echo requestId.
			if exchange == "BINANCE-FUT" {
				reply["response"] = map[string]any{"type": "error", "error": map[string]any{"code": 400, "message": "Unknown symbol"}}
			}
			delete(request, "requestId")
			data, err := json.Marshal(reply)
			if err != nil {
				t.Error(err)
				return
			}
			if err := connection.Write(r.Context(), websocket.MessageText, data); err != nil {
				return
			}
		}
	}))
	defer server.Close()
	tiger := NewTiger()
	defer tiger.Close()
	settings := core.DefaultSettings()
	settings.TigerPort = tigerPort(t, server.URL)
	c := &Controller{tiger: tiger, settings: settings}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.openInTerminal(ctx, "REZUSDT", "tiger", "binance", "bybit", "okx", "gate"); err != nil {
		t.Fatal(err)
	}
	exchangeMu.Lock()
	defer exchangeMu.Unlock()
	if !reflect.DeepEqual(exchanges, []string{"BINANCE-FUT", "BYBIT"}) {
		t.Fatalf("Tiger priority: %v", exchanges)
	}
}
