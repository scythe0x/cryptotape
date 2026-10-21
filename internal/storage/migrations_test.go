package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"cryptotape/internal/core"
)

func TestMigratesLegacyRulesAndKeepsAlertHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE rules (id INTEGER PRIMARY KEY,name TEXT NOT NULL,percent REAL NOT NULL,window_seconds INTEGER NOT NULL,sound TEXT NOT NULL,open_terminal INTEGER NOT NULL,enabled INTEGER NOT NULL)`,
		`CREATE TABLE settings (id INTEGER PRIMARY KEY,payload TEXT NOT NULL)`,
		`CREATE TABLE alerts (id INTEGER PRIMARY KEY,rule_id INTEGER NOT NULL,rule_name TEXT NOT NULL,symbol TEXT NOT NULL,direction TEXT NOT NULL,change_percent REAL NOT NULL,price REAL NOT NULL,spread_percent REAL NOT NULL,quote_volume REAL NOT NULL,event_time_ms INTEGER NOT NULL,created_at_ms INTEGER NOT NULL,sound TEXT NOT NULL,open_terminal INTEGER NOT NULL,terminal_status TEXT NOT NULL,terminal_error TEXT NOT NULL)`,
		`INSERT INTO settings VALUES(1,'{"minQuoteVolume":123000000,"cooldownSeconds":60,"proxy":"","monitorId":""}')`,
		`INSERT INTO rules VALUES(1,'legacy',3,60,'chime',1,0)`,
		`INSERT INTO alerts VALUES(1,1,'legacy','BTCUSDT','down',-3,90,0.01,150000000,9999999999999,9999999999999,'chime',1,'ok','')`,
	}
	for _, statement := range statements {
		if _, err = db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	rules, err := store.Rules()
	if err != nil || len(rules) != 1 {
		t.Fatalf("rules: %+v %v", rules, err)
	}
	rule := rules[0]
	if rule.BannerSeconds != core.BannerUntilClosed || rule.MinQuoteVolume != 123000000 || rule.Volume != 1 || len(rule.Terminals) != 1 || rule.Terminals[0] != "vataga" {
		t.Fatalf("lost legacy settings: %+v", rule)
	}
	settings, err := store.Settings()
	if err != nil || settings.TigerPort != 7819 || settings.TigerLinkGroup != "A" {
		t.Fatalf("Tiger defaults: %+v %v", settings, err)
	}
	alerts, err := store.ListAlerts(context.Background(), 0, 100)
	if err != nil || len(alerts) != 1 || alerts[0].BannerSeconds != core.BannerUntilClosed || alerts[0].TerminalStatus != "ok" {
		t.Fatalf("history: %+v %v", alerts, err)
	}
	rule.Color = "#46c6df"
	rule.Volume = .25
	rule.Terminals = []string{"vataga", "tiger"}
	rule.MinQuoteVolume = 99000000
	saved, err := store.SaveRule(rule)
	if err != nil {
		t.Fatal(err)
	}
	rules, err = store.Rules()
	if err != nil || rules[0].Color != saved.Color || rules[0].Volume != .25 || len(rules[0].Terminals) != 2 {
		t.Fatalf("new settings not persisted: %+v %v", rules, err)
	}
}

func TestRuleValidation(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "rules.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	valid := core.Rule{Name: "knife", Percent: 3, WindowSeconds: 60, Sound: "chime", Color: "#a78bfa", Volume: .7, MinQuoteVolume: 80e6, Terminals: []string{"tiger"}}
	for _, change := range []func(*core.Rule){
		func(r *core.Rule) { r.Volume = -.1 }, func(r *core.Rule) { r.Volume = 1.1 }, func(r *core.Rule) { r.Color = "red" },
		func(r *core.Rule) { r.MinQuoteVolume = -1 }, func(r *core.Rule) { r.Terminals = []string{"unknown"} }, func(r *core.Rule) { r.Terminals = []string{"tiger", "tiger"} },
	} {
		rule := valid
		change(&rule)
		if _, err := store.SaveRule(rule); err == nil {
			t.Fatalf("accepted invalid rule %+v", rule)
		}
	}
}

func TestDeletedRuleIDIsNotReusedByHistoryBadges(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "history.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	rules, err := store.Rules()
	if err != nil {
		t.Fatal(err)
	}
	last := rules[len(rules)-1]
	if _, err = store.InsertAlert(core.Candidate{Rule: last, Symbol: "BTCUSDT"}); err != nil {
		t.Fatal(err)
	}
	if err = store.DeleteRule(last.ID); err != nil {
		t.Fatal(err)
	}
	last.ID = 0
	saved, err := store.SaveRule(last)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID <= rules[len(rules)-1].ID {
		t.Fatal("a new rule inherited an old rule's history")
	}
}

func TestMigratesVersionOneWithoutLosingAlerts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v1.sqlite")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	alert, err := store.InsertAlert(core.Candidate{Rule: core.Rule{ID: 1, Name: "existing"}, Symbol: "BTCUSDT", Direction: "down", ChangePercent: -10})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		"ALTER TABLE alerts DROP COLUMN received_at_ms", "ALTER TABLE alerts DROP COLUMN detected_at_ms", "ALTER TABLE alerts DROP COLUMN write_started_at_ms",
		"ALTER TABLE rules DROP COLUMN open_exchanges", "ALTER TABLE alerts DROP COLUMN open_exchanges",
		"ALTER TABLE rules DROP COLUMN banner_seconds", "ALTER TABLE alerts DROP COLUMN banner_seconds",
		"ALTER TABLE alerts DROP COLUMN kind", "ALTER TABLE alerts DROP COLUMN listing", "ALTER TABLE rules DROP COLUMN kind",
		"ALTER TABLE rules DROP COLUMN listing_exchanges", "ALTER TABLE rules DROP COLUMN listing_types", "DROP TABLE listing_receipts", "PRAGMA user_version=1",
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
	alerts, err := reopened.ListAlerts(context.Background(), 0, 100)
	if err != nil || len(alerts) != 1 || alerts[0].ID != alert.ID || alerts[0].ChangePercent != -10 || alerts[0].ReceivedAtMs != 0 {
		t.Fatalf("v1 migration lost history: %+v %v", alerts, err)
	}
}
