package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"cryptotape/internal/core"
)

type Store struct{ db *sql.DB }

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA busy_timeout=3000`,
		`CREATE TABLE IF NOT EXISTS rules (
			id INTEGER PRIMARY KEY, name TEXT NOT NULL, percent REAL NOT NULL,
			window_seconds INTEGER NOT NULL, sound TEXT NOT NULL,
			open_terminal INTEGER NOT NULL, enabled INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS settings (id INTEGER PRIMARY KEY CHECK (id=1), payload TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS alerts (
			id INTEGER PRIMARY KEY, rule_id INTEGER NOT NULL, rule_name TEXT NOT NULL,
			symbol TEXT NOT NULL, direction TEXT NOT NULL, change_percent REAL NOT NULL,
			price REAL NOT NULL, spread_percent REAL NOT NULL, quote_volume REAL NOT NULL,
			event_time_ms INTEGER NOT NULL, created_at_ms INTEGER NOT NULL,
			sound TEXT NOT NULL, open_terminal INTEGER NOT NULL,
			terminal_status TEXT NOT NULL, terminal_error TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS alerts_created_at ON alerts(created_at_ms)`,
	} {
		if _, err = db.Exec(statement); err != nil {
			db.Close()
			return nil, err
		}
	}
	store := &Store{db: db}
	if err = store.seed(); err != nil {
		db.Close()
		return nil, err
	}
	_, err = db.Exec(`DELETE FROM alerts WHERE created_at_ms < ?`, time.Now().Add(-30*24*time.Hour).UnixMilli())
	return store, err
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) seed() error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM settings`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		for _, rule := range []core.Rule{
			{Name: "soft move", Percent: 3, WindowSeconds: 60, Sound: "chime", Enabled: true},
			{Name: "strong move", Percent: 10, WindowSeconds: 300, Sound: "alarm", OpenTerminal: true, Enabled: true},
		} {
			if _, err := s.SaveRule(rule); err != nil {
				return err
			}
		}
	}
	settings, err := json.Marshal(core.DefaultSettings())
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT OR IGNORE INTO settings(id, payload) VALUES (1, ?)`, string(settings))
	return err
}

func (s *Store) Rules() ([]core.Rule, error) {
	rows, err := s.db.Query(`SELECT id, name, percent, window_seconds, sound, open_terminal, enabled FROM rules ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules := make([]core.Rule, 0)
	for rows.Next() {
		var rule core.Rule
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.Percent, &rule.WindowSeconds, &rule.Sound, &rule.OpenTerminal, &rule.Enabled); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (s *Store) SaveRule(rule core.Rule) (core.Rule, error) {
	rule.Name = strings.TrimSpace(rule.Name)
	if rule.Name == "" || len(rule.Name) > 60 || rule.Percent < 0.1 || rule.Percent > 100 || rule.WindowSeconds < 5 || rule.WindowSeconds > 600 || rule.Sound == "" {
		return rule, errors.New("invalid alert rule")
	}
	if rule.ID == 0 {
		result, err := s.db.Exec(`INSERT INTO rules(name, percent, window_seconds, sound, open_terminal, enabled) VALUES (?, ?, ?, ?, ?, ?)`,
			rule.Name, rule.Percent, rule.WindowSeconds, rule.Sound, rule.OpenTerminal, rule.Enabled)
		if err != nil {
			return rule, err
		}
		rule.ID, err = result.LastInsertId()
		return rule, err
	}
	result, err := s.db.Exec(`UPDATE rules SET name=?, percent=?, window_seconds=?, sound=?, open_terminal=?, enabled=? WHERE id=?`,
		rule.Name, rule.Percent, rule.WindowSeconds, rule.Sound, rule.OpenTerminal, rule.Enabled, rule.ID)
	if err != nil {
		return rule, err
	}
	changed, err := result.RowsAffected()
	if err == nil && changed == 0 {
		err = sql.ErrNoRows
	}
	return rule, err
}

func (s *Store) DeleteRule(id int64) error {
	_, err := s.db.Exec(`DELETE FROM rules WHERE id=?`, id)
	return err
}

func (s *Store) Settings() (core.Settings, error) {
	var payload string
	var settings core.Settings
	if err := s.db.QueryRow(`SELECT payload FROM settings WHERE id=1`).Scan(&payload); err != nil {
		return settings, err
	}
	err := json.Unmarshal([]byte(payload), &settings)
	return settings, err
}

func (s *Store) SaveSettings(settings core.Settings) error {
	if settings.MinQuoteVolume < 0 || settings.CooldownSeconds < 0 || settings.CooldownSeconds > 3600 {
		return errors.New("invalid settings")
	}
	if settings.Proxy != "" {
		host, port, err := net.SplitHostPort(settings.Proxy)
		portNumber, portErr := strconv.Atoi(port)
		if err != nil || net.ParseIP(host) == nil || portErr != nil || portNumber < 1 || portNumber > 65535 {
			return errors.New("proxy must be ip:port")
		}
	}
	payload, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE settings SET payload=? WHERE id=1`, string(payload))
	return err
}

func (s *Store) InsertAlert(candidate core.Candidate) (core.Alert, error) {
	status := "skipped"
	if candidate.Rule.OpenTerminal {
		status = "pending"
	}
	alert := core.Alert{
		RuleID: candidate.Rule.ID, RuleName: candidate.Rule.Name, Symbol: candidate.Symbol,
		Direction: candidate.Direction, ChangePercent: candidate.ChangePercent,
		Price: candidate.Price, SpreadPercent: candidate.SpreadPercent,
		QuoteVolume: candidate.QuoteVolume, EventTimeMs: candidate.EventTime.UnixMilli(),
		CreatedAtMs: time.Now().UnixMilli(), Sound: candidate.Rule.Sound,
		OpenTerminal: candidate.Rule.OpenTerminal, TerminalStatus: status,
	}
	result, err := s.db.Exec(`INSERT INTO alerts(rule_id, rule_name, symbol, direction, change_percent, price, spread_percent,
		quote_volume, event_time_ms, created_at_ms, sound, open_terminal, terminal_status, terminal_error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '')`,
		alert.RuleID, alert.RuleName, alert.Symbol, alert.Direction, alert.ChangePercent, alert.Price,
		alert.SpreadPercent, alert.QuoteVolume, alert.EventTimeMs, alert.CreatedAtMs,
		alert.Sound, alert.OpenTerminal, alert.TerminalStatus)
	if err != nil {
		return alert, err
	}
	alert.ID, err = result.LastInsertId()
	return alert, err
}

func (s *Store) UpdateTerminal(id int64, status, detail string) error {
	_, err := s.db.Exec(`UPDATE alerts SET terminal_status=?, terminal_error=? WHERE id=?`, status, detail, id)
	return err
}

func (s *Store) ListAlerts(ctx context.Context, before int64, limit int) ([]core.Alert, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	if before == 0 {
		before = 1<<63 - 1
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, rule_id, rule_name, symbol, direction, change_percent, price,
		spread_percent, quote_volume, event_time_ms, created_at_ms, sound, open_terminal,
		terminal_status, terminal_error FROM alerts WHERE id < ? ORDER BY id DESC LIMIT ?`, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	alerts := make([]core.Alert, 0, limit)
	for rows.Next() {
		var alert core.Alert
		if err := rows.Scan(&alert.ID, &alert.RuleID, &alert.RuleName, &alert.Symbol, &alert.Direction,
			&alert.ChangePercent, &alert.Price, &alert.SpreadPercent, &alert.QuoteVolume,
			&alert.EventTimeMs, &alert.CreatedAtMs, &alert.Sound, &alert.OpenTerminal,
			&alert.TerminalStatus, &alert.TerminalError); err != nil {
			return nil, err
		}
		alerts = append(alerts, alert)
	}
	return alerts, rows.Err()
}

func (s *Store) AlertsAfter(ctx context.Context, after int64, limit int) ([]core.Alert, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, rule_id, rule_name, symbol, direction, change_percent, price,
		spread_percent, quote_volume, event_time_ms, created_at_ms, sound, open_terminal,
		terminal_status, terminal_error FROM alerts WHERE id > ? ORDER BY id ASC LIMIT ?`, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	alerts := make([]core.Alert, 0, limit)
	for rows.Next() {
		var alert core.Alert
		if err := rows.Scan(&alert.ID, &alert.RuleID, &alert.RuleName, &alert.Symbol, &alert.Direction,
			&alert.ChangePercent, &alert.Price, &alert.SpreadPercent, &alert.QuoteVolume,
			&alert.EventTimeMs, &alert.CreatedAtMs, &alert.Sound, &alert.OpenTerminal,
			&alert.TerminalStatus, &alert.TerminalError); err != nil {
			return nil, err
		}
		alerts = append(alerts, alert)
	}
	return alerts, rows.Err()
}

func DefaultPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user config directory: %w", err)
	}
	return filepath.Join(root, "cryptotape", "cryptotape.sqlite"), nil
}
