package storage

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"cryptotape/internal/core"
)

func TestListingExchangePriorityPersistsAndValidates(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "priority.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	rule := core.Rule{Kind: core.Listing, Name: "listing", Sound: "none", Color: "#ffffff", ListingTypes: []string{"spot"}, OpenExchanges: []string{"bybit", "gate", "okx", "binance"}}
	saved, err := store.SaveRule(rule)
	if err != nil {
		t.Fatal(err)
	}
	rule = saved
	rules, err := store.Rules()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rules[len(rules)-1].OpenExchanges, rule.OpenExchanges) {
		t.Fatal("priority reordered")
	}
	alert, err := store.InsertAlert(core.Candidate{Rule: rule, Symbol: "REZUSDT", Listing: &core.ListingDetails{Asset: "REZ", Exchange: "upbit", MarketType: "spot"}})
	if err != nil {
		t.Fatal(err)
	}
	rule.OpenExchanges = core.DefaultOpenExchanges()
	if _, err := store.SaveRule(rule); err != nil {
		t.Fatal(err)
	}
	history, err := store.AlertsAfter(context.Background(), alert.ID-1, 100)
	if err != nil || len(history) != 1 || !reflect.DeepEqual(history[0].OpenExchanges, saved.OpenExchanges) {
		t.Fatalf("alert lost original priority: %+v %v", history, err)
	}
	for _, priority := range [][]string{{"binance"}, {"binance", "binance", "okx", "gate"}, {"binance", "bybit", "okx", "invalid"}} {
		rule.OpenExchanges = priority
		if _, err := store.SaveRule(rule); err == nil {
			t.Fatalf("invalid priority accepted: %v", priority)
		}
	}
}

func TestVersionFourMigrationPreservesListingPriorityDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v4.sqlite")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	rule, err := store.SaveRule(core.Rule{Kind: core.Listing, Name: "old listing", Sound: "none", Color: "#ffffff", ListingTypes: []string{"spot"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertAlert(core.Candidate{Rule: rule, Symbol: "REZUSDT", Listing: &core.ListingDetails{Asset: "REZ", Exchange: "upbit", MarketType: "spot"}}); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{"ALTER TABLE rules DROP COLUMN open_exchanges", "ALTER TABLE alerts DROP COLUMN open_exchanges", "PRAGMA user_version=4"} {
		if _, err := store.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	rules, err := reopened.Rules()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rules[len(rules)-1].OpenExchanges, core.DefaultOpenExchanges()) {
		t.Fatal("legacy rule priority lost")
	}
	alerts, err := reopened.ListAlerts(context.Background(), 0, 100)
	if err != nil || len(alerts) != 1 || !reflect.DeepEqual(alerts[0].OpenExchanges, core.DefaultOpenExchanges()) {
		t.Fatalf("legacy alert priority lost: %+v %v", alerts, err)
	}
}
