package storage

import (
	"context"
	"path/filepath"
	"testing"

	"cryptotape/internal/core"
)

func TestBannerDurationPersistsInRulesAndAlertHistory(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "banners.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	rules, err := store.Rules()
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range rules {
		if rule.BannerSeconds != 15 {
			t.Fatal("new rules should default to 15 seconds")
		}
	}
	rule := rules[0]
	for _, seconds := range []int{0, 5, 15, 30, 60, 300, core.BannerUntilClosed} {
		rule.BannerSeconds = seconds
		if _, err := store.SaveRule(rule); err != nil {
			t.Fatal(err)
		}
		saved, err := store.Rules()
		if err != nil || saved[0].BannerSeconds != seconds {
			t.Fatalf("rule duration %d: %v", seconds, err)
		}
		alert, err := store.InsertAlert(core.Candidate{Rule: saved[0], Symbol: "BTCUSDT"})
		if err != nil || alert.BannerSeconds != seconds {
			t.Fatalf("alert duration %d: %v", seconds, err)
		}
		history, err := store.AlertsAfter(context.Background(), alert.ID-1, 100)
		if err != nil || len(history) != 1 || history[0].BannerSeconds != seconds {
			t.Fatalf("history duration %d: %+v %v", seconds, history, err)
		}
	}
	// changing the rule must not rewrite the duration of previous alerts.
	history, err := store.ListAlerts(context.Background(), 0, 100)
	if err != nil || len(history) != 7 || history[6].BannerSeconds != 0 || history[1].BannerSeconds != 300 {
		t.Fatalf("alert duration snapshot lost: %+v %v", history, err)
	}
	for _, seconds := range []int{-2, 1, 10, 301} {
		rule.BannerSeconds = seconds
		if _, err := store.SaveRule(rule); err == nil {
			t.Fatalf("accepted unsupported banner duration %d", seconds)
		}
	}
}

func TestMigratesVersionThreeBannerDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v3.sqlite")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := store.Rules()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertAlert(core.Candidate{Rule: rules[0], Symbol: "BTCUSDT"}); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		"ALTER TABLE rules DROP COLUMN open_exchanges", "ALTER TABLE alerts DROP COLUMN open_exchanges",
		"ALTER TABLE rules DROP COLUMN banner_seconds",
		"ALTER TABLE alerts DROP COLUMN banner_seconds",
		"PRAGMA user_version=3",
	} {
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
	rules, err = reopened.Rules()
	if err != nil || rules[0].BannerSeconds != core.BannerUntilClosed {
		t.Fatalf("v3 rule defaults: %+v %v", rules, err)
	}
	alerts, err := reopened.ListAlerts(context.Background(), 0, 100)
	if err != nil || len(alerts) != 1 || alerts[0].BannerSeconds != core.BannerUntilClosed {
		t.Fatalf("v3 history defaults: %+v %v", alerts, err)
	}
}
