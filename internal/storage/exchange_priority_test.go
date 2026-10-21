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
	for _, priority := range [][]string{{"binance", "bybit", "okx", "gate", "binance"}, {"binance", "binance", "okx", "gate"}, {"binance", "bybit", "okx", "invalid"}} {
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

func TestAllRuleKindsPersistEnabledExchangeSubsets(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "subsets.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, kind := range []core.RuleKind{core.PriceMove, core.Listing} {
		for _, exchanges := range [][]string{{"gate", "okx"}, {}} {
			rule := core.Rule{Kind: kind, Name: "subset", Sound: "none", Color: "#ffffff", Percent: 5, WindowSeconds: 60, ListingTypes: []string{"spot"}, OpenExchanges: exchanges}
			saved, err := store.SaveRule(rule)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(saved.OpenExchanges, exchanges) {
				t.Fatalf("changed enabled exchanges: %#v", saved.OpenExchanges)
			}
			rules, err := store.Rules()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(rules[len(rules)-1].OpenExchanges, exchanges) {
				t.Fatalf("lost enabled exchanges: %+v", rules)
			}
		}
	}
}

func TestVersionFiveMigrationEnablesAllPriceExchanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v5.sqlite")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	price := core.Rule{Name: "price", Percent: 5, WindowSeconds: 60, Sound: "none", Color: "#ffffff"}
	rule, err := store.SaveRule(price)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertAlert(core.Candidate{Rule: rule, Symbol: "BTCUSDT"}); err != nil {
		t.Fatal(err)
	}
	listing, err := store.SaveRule(core.Rule{Kind: core.Listing, Name: "listing", Sound: "none", Color: "#ffffff", ListingTypes: []string{"spot"}, OpenExchanges: []string{"gate", "okx", "bybit", "binance"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{`UPDATE rules SET open_exchanges='[]' WHERE kind='price_move'`, `UPDATE alerts SET open_exchanges='[]' WHERE kind='price_move'`, `PRAGMA user_version=5`} {
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
	for _, saved := range rules {
		want := core.DefaultOpenExchanges()
		if saved.ID == listing.ID {
			want = listing.OpenExchanges
		}
		if !reflect.DeepEqual(saved.OpenExchanges, want) {
			t.Fatalf("migration changed order: %+v", saved)
		}
	}
	alerts, err := reopened.ListAlerts(context.Background(), 0, 10)
	if err != nil || len(alerts) != 1 || !reflect.DeepEqual(alerts[0].OpenExchanges, core.DefaultOpenExchanges()) {
		t.Fatalf("historical defaults: %+v %v", alerts, err)
	}
}
