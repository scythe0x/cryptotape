package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"cryptotape/internal/core"
)

func TestListingRuleRoundTripAndAtomicPersistentDedup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "listings.sqlite")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	rule, err := store.SaveRule(core.Rule{Kind: core.Listing, Name: "external", Sound: "none", Color: "#a78bfa", ListingTypes: []string{"spot"}, ListingExchanges: []string{"upbit"}})
	if err != nil {
		t.Fatal(err)
	}
	rules, err := store.Rules()
	if err != nil || !rules[len(rules)-1].IsListing() || rules[len(rules)-1].Percent != 0 {
		t.Fatalf("listing rule: %+v %v", rules, err)
	}
	now := time.Now()
	details := core.ListingDetails{Exchange: "upbit", Asset: "REZ", MarketType: "spot", Markets: []string{"krw"}, URL: "https://upbit.com/notice?id=1", DetectedTimeUs: now.Add(-3 * time.Second).UnixMicro(), SentTimeUs: now.UnixMicro()}
	candidate := core.Candidate{Rule: rule, Listing: &details, Symbol: "REZUSDT", Received: now, Detected: now, EventTime: now.Add(-3 * time.Second)}
	price := core.Candidate{Rule: core.Rule{ID: 1}, Symbol: "BTCUSDT", Direction: "down", ChangePercent: -5}
	alerts, err := store.InsertAlerts(context.Background(), []core.Candidate{candidate, candidate, price})
	if err != nil || len(alerts) != 3 || alerts[0].ID == 0 || alerts[1].ID != 0 || alerts[2].Symbol != "BTCUSDT" {
		t.Fatalf("batch positions: %+v %v", alerts, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	details.URL = "https://x.com/upbit/status/2"
	duplicate, err := store.InsertAlert(candidate)
	if err != nil || duplicate.ID != 0 {
		t.Fatalf("duplicate reappeared after restart: %+v %v", duplicate, err)
	}
	saved, err := store.AlertsAfter(context.Background(), 0, 200)
	if err != nil || len(saved) != 2 || saved[0].Kind != core.Listing || saved[0].Listing == nil || saved[0].Listing.DetectedTimeUs != details.DetectedTimeUs || saved[1].Kind != core.PriceMove {
		t.Fatalf("history: %+v %v", saved, err)
	}
	details.Markets = []string{"usdt"}
	distinct, err := store.InsertAlert(candidate)
	if err != nil || distinct.ID == 0 {
		t.Fatalf("new quote market suppressed: %+v %v", distinct, err)
	}
	details.Asset = "OTHER"
	invalid := candidate
	invalid.Rule.Kind = core.PriceMove
	if _, err := store.InsertAlerts(context.Background(), []core.Candidate{candidate, invalid}); err == nil {
		t.Fatal("invalid batch accepted")
	}
	retry, err := store.InsertAlert(candidate)
	if err != nil || retry.ID == 0 {
		t.Fatalf("rolled back receipt prevented retry: %+v %v", retry, err)
	}
}

func TestListingFiltersCannotSubscribeToBinanceOrUnknownKinds(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "validation.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	valid := core.Rule{Kind: core.Listing, Name: "external", Sound: "none", Color: "#a78bfa", ListingTypes: []string{"spot"}}
	for _, change := range []func(*core.Rule){
		func(r *core.Rule) { r.ListingTypes = nil },
		func(r *core.Rule) { r.ListingTypes = []string{"caution"} },
		func(r *core.Rule) { r.ListingExchanges = []string{"binance"} },
		func(r *core.Rule) { r.Kind = "unknown" },
	} {
		rule := valid
		change(&rule)
		if _, err := store.SaveRule(rule); err == nil {
			t.Fatalf("invalid filter accepted: %+v", rule)
		}
	}
}
