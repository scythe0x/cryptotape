package desktop

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"cryptotape/internal/core"
	"cryptotape/internal/listings"
)

func externalEvent(t *testing.T, at time.Time, exchange string, assets ...string) listings.Event {
	t.Helper()
	var event listings.Event
	if err := json.Unmarshal([]byte(`{"type":"announcement","url":"https://upbit.com/notice?id=1","parser":{"exchange":"upbit","classification":{"event":"listing","type":"spot","category":"crypto"}}}`), &event); err != nil {
		t.Fatal(err)
	}
	event.Parser.Exchange = exchange
	for _, asset := range assets {
		event.Parser.Assets = append(event.Parser.Assets, struct {
			Symbol string `json:"symbol"`
		}{Symbol: asset})
	}
	event.Detected, event.Sent = at.Add(-3*time.Second).UnixMicro(), at.UnixMicro()
	return event
}

func TestExternalListingPipelineMatchesContractsAndDispatchesOnce(t *testing.T) {
	published := make(chan core.Alert, 10)
	c, _ := testController(t, func(name string, data any) {
		if name == "alert:new" {
			published <- data.(core.Alert)
		}
	})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { requests.Add(1); fmt.Fprint(w, `{"status":"ok"}`) }))
	defer server.Close()
	c.terminal.baseURL = server.URL
	rule, err := c.SaveRule(core.Rule{Kind: core.Listing, Name: "external", Sound: "none", Color: "#a78bfa", ListingTypes: []string{"spot"}, Terminals: []string{"vataga"}})
	if err != nil {
		t.Fatal(err)
	}
	c.updateListingCatalog([]string{"REZUSDT", "1000PEPEUSDT", "1000AMBIGUSDT", "AMBIGUSDT"})
	at := time.Now()
	event := externalEvent(t, at, "upbit", "REZ", "PEPE", "UNKNOWN", "AMBIG")
	c.onListing(event, at)
	c.onListing(event, at)
	c.onListing(externalEvent(t, at, "binance", "REZ"), at)
	c.actionWG.Wait()
	if len(published) != 3 {
		t.Fatalf("published %d alerts", len(published))
	}
	if requests.Load() != 3 {
		t.Fatalf("duplicate auto-open: %d", requests.Load())
	}
	for range 3 {
		alert := <-published
		if alert.Kind != core.Listing || alert.RuleID != rule.ID || alert.Direction != "" || alert.Listing == nil || alert.Listing.Exchange != "upbit" {
			t.Fatalf("invalid listing card: %+v", alert)
		}
	}
	if c.ListingSource().Health.Unmatched != 2 {
		t.Fatal("unmatched assets not diagnosed")
	}
	c.engine.SetVolumes(map[string]float64{"REZUSDT": 1})
	// listing rules do not create price windows or subscribe all markets by turnover.
	if len(c.engine.Process(marketQuote("REZUSDT", 100, at))) != 0 || len(c.engine.Process(marketQuote("REZUSDT", 80, at.Add(time.Second)))) != 0 || core.MinimumVolume([]core.Rule{rule}) < 1e99 {
		t.Fatal("listing rule affected price detector")
	}
}

func TestListingFiltersAndStaleEventsDoNotStopPriceAlerts(t *testing.T) {
	published := make(chan core.Alert, 10)
	c, _ := testController(t, func(name string, data any) {
		if name == "alert:new" {
			published <- data.(core.Alert)
		}
	})
	_, err := c.SaveRule(core.Rule{Kind: core.Listing, Name: "upbit", Sound: "none", Color: "#a78bfa", ListingTypes: []string{"spot"}, ListingExchanges: []string{"upbit"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.SaveRule(lifecycleRule()); err != nil {
		t.Fatal(err)
	}
	c.updateListingCatalog([]string{"REZUSDT"})
	at := time.Now()
	c.onListing(externalEvent(t, at, "bithumb", "REZ"), at)
	old := externalEvent(t, at, "upbit", "REZ")
	old.Detected = at.Add(-11 * time.Second).UnixMicro()
	c.onListing(old, at)
	c.engine.SetVolumes(map[string]float64{"REZUSDT": 100e6})
	c.onQuote(marketQuote("REZUSDT", 100, at.Add(-time.Second)))
	c.onQuote(marketQuote("REZUSDT", 94, at))
	c.actionWG.Wait()
	if len(published) != 1 || (<-published).Kind != core.PriceMove || c.ListingSource().Health.Rejected != 1 {
		t.Fatal("listing source failure affected price alerts")
	}
}

func TestListingKeyIsNotReturnedToWebviews(t *testing.T) {
	c, _ := testController(t, func(string, any) {})
	if err := c.SaveListingSource(listings.FreeEndpoint, "unit-test-secret"); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(c.ListingSource())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "unit-test-secret") || !c.ListingSource().KeyConfigured {
		t.Fatal("key exposed or not stored")
	}
	if err := c.SaveListingSource("wss://example.com/v2/full", ""); err == nil {
		t.Fatal("key could be forwarded to arbitrary endpoint")
	}
	if err := c.ClearListingKey(); err != nil || c.ListingSource().KeyConfigured {
		t.Fatal("cannot remove key")
	}
}
