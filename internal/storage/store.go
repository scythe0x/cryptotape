package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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
	if err = migrate(db); err != nil {
		db.Close()
		return nil, err
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
			{Name: "soft move", Percent: 3, WindowSeconds: 60, Sound: "chime", Color: "#41c977", Volume: 1, MinQuoteVolume: 80_000_000, Terminals: []string{}},
			{Name: "strong move", Percent: 10, WindowSeconds: 300, Sound: "alarm", Color: "#fa423e", Volume: 1, MinQuoteVolume: 80_000_000, Terminals: []string{"vataga"}},
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
	rows, err := s.db.Query(`SELECT id, name, percent, window_seconds, sound, color, volume, min_quote_volume, terminals FROM rules ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules := make([]core.Rule, 0)
	for rows.Next() {
		var rule core.Rule
		var terminals string
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.Percent, &rule.WindowSeconds, &rule.Sound, &rule.Color, &rule.Volume, &rule.MinQuoteVolume, &terminals); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(terminals), &rule.Terminals); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (s *Store) SaveRule(rule core.Rule) (core.Rule, error) {
	rule.Name = strings.TrimSpace(rule.Name)
	if rule.Name == "" || utf8.RuneCountInString(rule.Name) > 60 || rule.Percent < 0.1 || rule.Percent > 100 || rule.WindowSeconds < 5 || rule.WindowSeconds > 600 || rule.Sound == "" || !validColor.MatchString(rule.Color) || !finite(rule.Percent) || !finite(rule.Volume) || rule.Volume < 0 || rule.Volume > 1 || !finite(rule.MinQuoteVolume) || rule.MinQuoteVolume < 0 || !validTerminals(rule.Terminals) {
		return rule, errors.New("invalid alert rule")
	}
	if rule.Terminals == nil {
		rule.Terminals = []string{}
	}
	terminals, err := json.Marshal(rule.Terminals)
	if err != nil {
		return rule, err
	}
	if rule.ID == 0 {
		result, err := s.db.Exec(`INSERT INTO rules(id,name,percent,window_seconds,sound,open_terminal,enabled,color,volume,min_quote_volume,terminals)
 VALUES ((SELECT MAX(value)+1 FROM (SELECT COALESCE(MAX(id),0) AS value FROM rules UNION ALL SELECT COALESCE(MAX(rule_id),0) AS value FROM alerts)),?,?,?,?,?,1,?,?,?,?)`,
			rule.Name, rule.Percent, rule.WindowSeconds, rule.Sound, len(rule.Terminals) > 0, rule.Color, rule.Volume, rule.MinQuoteVolume, string(terminals))
		if err != nil {
			return rule, err
		}
		rule.ID, err = result.LastInsertId()
		return rule, err
	}
	result, err := s.db.Exec(`UPDATE rules SET name=?, percent=?, window_seconds=?, sound=?, open_terminal=?, enabled=1,color=?,volume=?,min_quote_volume=?,terminals=? WHERE id=?`,
		rule.Name, rule.Percent, rule.WindowSeconds, rule.Sound, len(rule.Terminals) > 0, rule.Color, rule.Volume, rule.MinQuoteVolume, string(terminals), rule.ID)
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
	settings = core.DefaultSettings()
	if err := json.Unmarshal([]byte(payload), &settings); err != nil {
		return settings, err
	}
	if settings.Proxies == nil {
		settings.Proxies = []core.ProxyProfile{}
	}
	var legacy struct {
		Proxy string `json:"proxy"`
	}
	if err := json.Unmarshal([]byte(payload), &legacy); err != nil {
		return settings, err
	}
	if legacy.Proxy != "" && len(settings.Proxies) == 0 {
		endpoint, err := url.Parse("http://" + legacy.Proxy)
		if err != nil {
			return settings, errors.New("invalid legacy proxy")
		}
		port, err := strconv.Atoi(endpoint.Port())
		if err != nil {
			return settings, errors.New("invalid legacy proxy")
		}
		settings.Proxies = []core.ProxyProfile{{ID: "legacy", Name: "proxy-1", Host: endpoint.Hostname(), Port: port, Protocol: "http"}}
		settings.ActiveProxyID = "legacy"
	}
	return settings, nil
}

func (s *Store) SaveSettings(settings core.Settings) error {
	if settings.Language != "en" && settings.Language != "ru" {
		return errors.New("unsupported interface language")
	}
	if settings.CooldownSeconds < 0 || settings.CooldownSeconds > 3600 {
		return errors.New("invalid settings")
	}
	if settings.TigerPort < 1 || settings.TigerPort > 65535 || !validLinkGroup.MatchString(settings.TigerLinkGroup) {
		return errors.New("invalid Tiger port or link group")
	}
	if err := core.ValidateProxies(settings); err != nil {
		return err
	}
	payload, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE settings SET payload=? WHERE id=1`, string(payload))
	return err
}

func (s *Store) InsertAlert(candidate core.Candidate) (core.Alert, error) {
	alerts, err := s.InsertAlerts(context.Background(), []core.Candidate{candidate})
	if err != nil {
		return core.Alert{}, err
	}
	return alerts[0], nil
}

// publish only after the whole batch is committed.
func (s *Store) InsertAlerts(ctx context.Context, candidates []core.Candidate) ([]core.Alert, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	alerts := make([]core.Alert, 0, len(candidates))
	for _, candidate := range candidates {
		alert, err := insertAlert(ctx, tx, candidate)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, alert)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return alerts, nil
}

func insertAlert(ctx context.Context, tx *sql.Tx, candidate core.Candidate) (core.Alert, error) {
	status := "skipped"
	if len(candidate.Rule.Terminals) > 0 {
		status = "pending"
	}
	alert := core.Alert{
		ReceivedAtMs: timestampMs(candidate.Received), DetectedAtMs: timestampMs(candidate.Detected), WriteStartedAtMs: time.Now().UnixMilli(),
		RuleID: candidate.Rule.ID, RuleName: candidate.Rule.Name, Symbol: candidate.Symbol,
		Direction: candidate.Direction, ChangePercent: candidate.ChangePercent,
		Price: candidate.Price, SpreadPercent: candidate.SpreadPercent,
		QuoteVolume: candidate.QuoteVolume, EventTimeMs: candidate.EventTime.UnixMilli(),
		CreatedAtMs: timestampMs(candidate.Detected), Sound: candidate.Rule.Sound,
		Color: candidate.Rule.Color, Volume: candidate.Rule.Volume, Terminals: append([]string{}, candidate.Rule.Terminals...), TerminalStatus: status,
	}
	if alert.CreatedAtMs == 0 {
		alert.CreatedAtMs = time.Now().UnixMilli()
	}
	terminals, err := json.Marshal(alert.Terminals)
	if err != nil {
		return alert, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO alerts(rule_id, rule_name, symbol, direction, change_percent, price, spread_percent,
		quote_volume, event_time_ms, created_at_ms, sound, open_terminal, terminal_status, terminal_error,color,volume,terminals,received_at_ms,detected_at_ms,write_started_at_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '',?,?,?,?,?,?)`,
		alert.RuleID, alert.RuleName, alert.Symbol, alert.Direction, alert.ChangePercent, alert.Price,
		alert.SpreadPercent, alert.QuoteVolume, alert.EventTimeMs, alert.CreatedAtMs,
		alert.Sound, len(alert.Terminals) > 0, alert.TerminalStatus, alert.Color, alert.Volume, string(terminals), alert.ReceivedAtMs, alert.DetectedAtMs, alert.WriteStartedAtMs)
	if err != nil {
		return alert, err
	}
	alert.ID, err = result.LastInsertId()
	return alert, err
}

func (s *Store) UpdateTerminal(id int64, status, detail string) error {
	return s.UpdateTerminalContext(context.Background(), id, status, detail)
}

func (s *Store) UpdateTerminalContext(ctx context.Context, id int64, status, detail string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE alerts SET terminal_status=?, terminal_error=? WHERE id=?`, status, detail, id)
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
		spread_percent, quote_volume, event_time_ms, created_at_ms, sound, color, volume, terminals,
		terminal_status, terminal_error, received_at_ms, detected_at_ms, write_started_at_ms FROM alerts WHERE id < ? ORDER BY id DESC LIMIT ?`, before, limit)
	if err != nil {
		return nil, err
	}
	return scanAlerts(rows, limit)
}

func (s *Store) AlertsAfter(ctx context.Context, after int64, limit int) ([]core.Alert, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, rule_id, rule_name, symbol, direction, change_percent, price,
		spread_percent, quote_volume, event_time_ms, created_at_ms, sound, color, volume, terminals,
		terminal_status, terminal_error, received_at_ms, detected_at_ms, write_started_at_ms FROM alerts WHERE id > ? ORDER BY id ASC LIMIT ?`, after, limit)
	if err != nil {
		return nil, err
	}
	return scanAlerts(rows, limit)
}

func DefaultPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user config directory: %w", err)
	}
	return filepath.Join(root, "cryptotape", "cryptotape.sqlite"), nil
}

var validColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
var validLinkGroup = regexp.MustCompile(`^([A-Z]|[0-9]|1[0-8])$`)

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
func validTerminals(ids []string) bool {
	seen := map[string]bool{}
	for _, id := range ids {
		if (id != "vataga" && id != "tiger") || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

func scanAlerts(rows *sql.Rows, limit int) ([]core.Alert, error) {
	defer rows.Close()
	alerts := make([]core.Alert, 0, limit)
	for rows.Next() {
		var alert core.Alert
		var terminals string
		if err := rows.Scan(&alert.ID, &alert.RuleID, &alert.RuleName, &alert.Symbol, &alert.Direction,
			&alert.ChangePercent, &alert.Price, &alert.SpreadPercent, &alert.QuoteVolume,
			&alert.EventTimeMs, &alert.CreatedAtMs, &alert.Sound, &alert.Color, &alert.Volume, &terminals,
			&alert.TerminalStatus, &alert.TerminalError, &alert.ReceivedAtMs, &alert.DetectedAtMs, &alert.WriteStartedAtMs); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(terminals), &alert.Terminals); err != nil {
			return nil, err
		}
		if alert.Terminals == nil {
			alert.Terminals = []string{}
		}
		alerts = append(alerts, alert)
	}
	return alerts, rows.Err()
}

func timestampMs(at time.Time) int64 {
	if at.IsZero() {
		return 0
	}
	return at.UnixMilli()
}
