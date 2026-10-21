package listings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"cryptotape/internal/core"
)

func listingEvent(t *testing.T, at time.Time) Event {
	t.Helper()
	raw := `{"id":1,"type":"announcement","url":"https://upbit.com/service_center/notice?id=1","parser":{"exchange":"upbit","classification":{"event":"listing","type":"spot","category":"crypto","markets":["usdt","krw","krw"]},"assets":[{"symbol":"REZ"},{"symbol":"REZ"}]}}`
	var event Event
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatal(err)
	}
	event.Detected, event.Sent = at.Add(-3*time.Second).UnixMicro(), at.UnixMicro()
	return event
}

func TestClassifiedListingsRejectNoiseStaleAndInvalidData(t *testing.T) {
	at := time.Now()
	cases := []struct {
		name    string
		change  func(*Event)
		count   int
		invalid bool
	}{
		{"listing", func(*Event) {}, 1, false},
		{"tweet", func(e *Event) { e.Type = "tweet" }, 1, false},
		{"binance", func(e *Event) { e.Parser.Exchange = "binance" }, 0, false},
		{"delisting", func(e *Event) { e.Parser.Classification.Event = "delisting" }, 0, false},
		{"caution", func(e *Event) { e.Parser.Classification.Event = "caution" }, 0, false},
		{"unclassified", func(e *Event) { e.Parser.Classification.Event = "none" }, 0, false},
		{"rwa", func(e *Event) { e.Parser.Classification.Category = "rwa" }, 0, false},
		{"stale", func(e *Event) { e.Detected = at.Add(-11 * time.Second).UnixMicro() }, 0, true},
		{"future", func(e *Event) { e.Detected = at.Add(3 * time.Second).UnixMicro(); e.Sent = e.Detected }, 0, true},
		{"missing time", func(e *Event) { e.Detected = 0 }, 0, true},
		{"inverted time", func(e *Event) { e.Sent = e.Detected - 1 }, 0, true},
		{"unsafe url", func(e *Event) { e.URL = "javascript:alert(1)" }, 0, true},
		{"credentials in url", func(e *Event) { e.URL = "https://key@example.com/notice" }, 0, true},
		{"invalid symbol", func(e *Event) {
			for i := range e.Parser.Assets {
				e.Parser.Assets[i].Symbol = "BAD/USDT"
			}
		}, 0, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			event := listingEvent(t, at)
			test.change(&event)
			assets, err := event.Listings(at)
			if (err != nil) != test.invalid || len(assets) != test.count {
				t.Fatalf("assets=%+v err=%v", assets, err)
			}
			if len(assets) > 0 && !reflect.DeepEqual(assets[0].Markets, []string{"krw", "usdt"}) {
				t.Fatal("markets not normalized")
			}
		})
	}
}

func TestCatalogRequiresUniqueExistingContract(t *testing.T) {
	catalog := NewCatalog([]string{"REZUSDT", "1000PEPEUSDT", "REZUSDT", "BTCUSDC", "BAD/USDT", "REZUSDT_260925"})
	for asset, want := range map[string]string{"REZ": "REZUSDT", "PEPE": "1000PEPEUSDT", "1000PEPE": "1000PEPEUSDT", "BTC": "", "RE": ""} {
		got, ok := catalog.Resolve(asset)
		if got != want || ok != (want != "") {
			t.Fatalf("%s: %s %v", asset, got, ok)
		}
	}
	ambiguous := NewCatalog([]string{"PEPEUSDT", "1000PEPEUSDT"})
	if _, ok := ambiguous.Resolve("PEPE"); ok {
		t.Fatal("ambiguous ticker resolved")
	}
}

func TestSemanticDedupIgnoresEdgeAndSourceIDs(t *testing.T) {
	a := core.ListingDetails{Exchange: "upbit", Asset: "REZ", MarketType: "spot", Markets: []string{"krw", "usdt"}, URL: "https://upbit.com/notice/1"}
	b := a
	b.URL = "https://x.com/upbit/status/2"
	b.Markets = []string{"usdt", "krw", "krw"}
	if DedupKey(a) != DedupKey(b) {
		t.Fatal("duplicate source posts have different keys")
	}
	b.Markets = []string{"btc"}
	if DedupKey(a) == DedupKey(b) {
		t.Fatal("different quote market suppressed")
	}
}

func TestConfigIsPrivateAndRejectsUntrustedEndpoints(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.json")
	config := Config{Endpoint: FreeEndpoint, Key: "unit-test-key"}
	if err := SaveConfig(path, config); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadConfig(path)
	if err != nil || loaded != config {
		t.Fatalf("round trip: %+v %v", loaded, err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0077 != 0 {
			t.Fatal("key file is accessible to other users")
		}
	}
	for _, endpoint := range []string{"wss://example.com/v2/full", "wss://tokyo.newlistings.pro/v2/full", "wss://ny.newlistings.pro/v2/full", "wss://seoul.newlistings.pro/v2/full"} {
		config.Endpoint = endpoint
		if err := SaveConfig(path, config); err == nil {
			t.Fatal("non-free endpoint accepted")
		}
	}
	config.Endpoint = FreeEndpoint
	config.Key = "secret\r\nInjected: yes"
	if err := SaveConfig(path, config); err == nil {
		t.Fatal("invalid key accepted")
	}
}
