package desktop

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"cryptotape/internal/binance"
	"cryptotape/internal/core"
	"cryptotape/internal/storage"
)

type EventSink func(name string, data any)

type Controller struct {
	store    *storage.Store
	engine   *core.Engine
	audio    *Audio
	terminal *Terminal
	emit     EventSink

	mu       sync.RWMutex
	rules    []core.Rule
	settings core.Settings
	health   core.Health
	cancel   context.CancelFunc
	stop     context.CancelFunc
	root     context.Context
	launchID int64
	version  atomic.Uint64
	feedWG   sync.WaitGroup
	actionWG sync.WaitGroup
}

func NewController(path string, emit EventSink) (*Controller, error) {
	store, err := storage.Open(path)
	if err != nil {
		return nil, err
	}
	rules, err := store.Rules()
	if err != nil {
		store.Close()
		return nil, err
	}
	settings, err := store.Settings()
	if err != nil {
		store.Close()
		return nil, err
	}
	audio, err := NewAudio(filepath.Join(filepath.Dir(path), "sounds"))
	if err != nil {
		store.Close()
		return nil, err
	}
	latest, err := store.ListAlerts(context.Background(), 0, 1)
	if err != nil {
		store.Close()
		return nil, err
	}
	controller := &Controller{
		store: store, engine: core.NewEngine(rules, settings), audio: audio,
		terminal: NewTerminal(), emit: emit, rules: rules, settings: settings,
		health: core.Health{State: "starting", Detail: "initializing"},
	}
	if len(latest) > 0 {
		controller.launchID = latest[0].ID
	}
	return controller, nil
}

func (c *Controller) Start(ctx context.Context) {
	c.mu.Lock()
	c.root, c.stop = context.WithCancel(ctx)
	root := c.root
	c.mu.Unlock()
	c.feedWG.Add(2)
	go func() {
		defer c.feedWG.Done()
		if err := c.audio.Warm(); err != nil {
			slog.Warn("audio device", "error", err)
		}
	}()
	c.restartFeed()
	go func() {
		defer c.feedWG.Done()
		c.healthLoop(root)
	}()
}

func (c *Controller) Close() error {
	c.mu.Lock()
	if c.stop != nil {
		c.stop()
	}
	if c.cancel != nil {
		c.cancel()
	}
	c.mu.Unlock()
	c.feedWG.Wait()
	c.actionWG.Wait()
	return c.store.Close()
}

func (c *Controller) GetSnapshot() (core.Snapshot, error) {
	c.mu.RLock()
	snapshot := core.Snapshot{
		Rules: append([]core.Rule(nil), c.rules...), Settings: c.settings, Health: c.health,
	}
	c.mu.RUnlock()
	alerts, err := c.store.ListAlerts(context.Background(), 0, 100)
	snapshot.Alerts = alerts
	return snapshot, err
}

func (c *Controller) SaveRule(rule core.Rule) (core.Rule, error) {
	if !c.audio.Has(rule.Sound) {
		return rule, errors.New("unknown sound")
	}
	saved, err := c.store.SaveRule(rule)
	if err != nil {
		return saved, err
	}
	return saved, c.reloadRules()
}

func (c *Controller) DeleteRule(id int64) error {
	if err := c.store.DeleteRule(id); err != nil {
		return err
	}
	return c.reloadRules()
}

func (c *Controller) reloadRules() error {
	rules, err := c.store.Rules()
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.rules = rules
	settings := c.settings
	c.mu.Unlock()
	c.engine.Configure(rules, settings)
	c.emit("rules:changed", rules)
	return nil
}

func (c *Controller) SaveSettings(settings core.Settings) error {
	if err := c.store.SaveSettings(settings); err != nil {
		return err
	}
	c.mu.Lock()
	oldProxy := c.settings.Proxy
	oldVolume := c.settings.MinQuoteVolume
	c.settings = settings
	rules := append([]core.Rule(nil), c.rules...)
	c.mu.Unlock()
	c.engine.Configure(rules, settings)
	c.emit("settings:changed", settings)
	if oldProxy != settings.Proxy || oldVolume != settings.MinQuoteVolume {
		c.engine.ResetAll()
		c.restartFeed()
	}
	return nil
}

func (c *Controller) TestProxy(proxy string) error {
	feed, err := binance.NewFeed(proxy, 0, binance.Callbacks{})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	return feed.Test(ctx)
}

func (c *Controller) SoundIDs() []string { return c.audio.IDs() }

func (c *Controller) ImportSound(name, extension, encoded string) (string, error) {
	id, err := c.audio.Import(name, extension, encoded)
	if err == nil {
		c.emit("sounds:changed", c.audio.IDs())
	}
	return id, err
}

func (c *Controller) PreviewSound(id string) error { return c.audio.Play(id) }

func (c *Controller) ListAlerts(before int64, limit int) ([]core.Alert, error) {
	return c.store.ListAlerts(context.Background(), before, limit)
}

func (c *Controller) AlertsAfter(after int64, limit int) ([]core.Alert, error) {
	return c.store.AlertsAfter(context.Background(), after, limit)
}

func (c *Controller) SessionAlerts() ([]core.Alert, error) {
	alerts, err := c.store.ListAlerts(context.Background(), 0, 3)
	if err != nil {
		return nil, err
	}
	result := make([]core.Alert, 0, len(alerts))
	for _, alert := range alerts {
		if alert.ID > c.launchID {
			result = append(result, alert)
		}
	}
	slices.Reverse(result)
	return result, nil
}

func (c *Controller) OpenTerminal(id int64, symbol string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := c.terminal.Open(ctx, symbol)
	if id > 0 {
		status, detail := "manual-ok", ""
		if err != nil {
			status, detail = "manual-error", err.Error()
		}
		if saveErr := c.store.UpdateTerminal(id, status, detail); saveErr != nil {
			slog.Error("save terminal status", "error", saveErr)
		}
		c.emit("alert:update", map[string]any{"id": id, "terminalStatus": status, "terminalError": detail})
	}
	return err
}

func (c *Controller) restartFeed() {
	c.mu.Lock()
	if c.root == nil {
		c.mu.Unlock()
		return
	}
	if c.cancel != nil {
		c.cancel()
	}
	ctx, cancel := context.WithCancel(c.root)
	c.cancel = cancel
	proxy := c.settings.Proxy
	minVolume := c.settings.MinQuoteVolume
	version := c.version.Add(1)
	c.feedWG.Add(1)
	c.mu.Unlock()
	go func() {
		defer c.feedWG.Done()
		c.feedLoop(ctx, proxy, minVolume, version)
	}()
}

func (c *Controller) feedLoop(ctx context.Context, proxy string, minVolume float64, version uint64) {
	backoff := time.Second
	for ctx.Err() == nil {
		feed, err := binance.NewFeed(proxy, minVolume, binance.Callbacks{
			Quote: func(quote core.Quote) {
				if c.version.Load() == version {
					c.onQuote(quote)
				}
			},
			Volume: func(values map[string]float64) {
				if c.version.Load() == version {
					c.engine.SetVolumes(values)
				}
			},
			Gap: func(symbols []string) {
				if c.version.Load() != version {
					return
				}
				if symbols == nil {
					c.engine.ResetAll()
				} else {
					c.engine.Reset(symbols)
				}
			},
			Health: func(health core.Health) {
				if c.version.Load() == version {
					c.setHealth(health)
				}
			},
		})
		if err == nil {
			err = feed.Run(ctx)
		}
		if ctx.Err() != nil {
			return
		}
		c.setHealth(core.Health{State: "error", Detail: err.Error(), UpdatedAtMs: time.Now().UnixMilli()})
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

func (c *Controller) onQuote(quote core.Quote) {
	for _, candidate := range c.engine.Process(quote) {
		alert, err := c.store.InsertAlert(candidate)
		if err != nil {
			c.setHealth(core.Health{State: "error", Detail: "alert storage: " + err.Error(), UpdatedAtMs: time.Now().UnixMilli()})
			return
		}
		c.emit("alert:new", alert)
		c.actionWG.Add(1)
		go func() {
			defer c.actionWG.Done()
			if err := c.audio.Play(alert.Sound); err != nil {
				slog.Error("play alert", "error", err)
			}
		}()
		if alert.OpenTerminal {
			c.actionWG.Add(1)
			go func() {
				defer c.actionWG.Done()
				c.openAutomatically(alert)
			}()
		}
	}
}

func (c *Controller) openAutomatically(alert core.Alert) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := c.terminal.Open(ctx, alert.Symbol)
	status, detail := "ok", ""
	if err != nil {
		status, detail = "error", err.Error()
	}
	if saveErr := c.store.UpdateTerminal(alert.ID, status, detail); saveErr != nil {
		slog.Error("save terminal status", "error", saveErr)
	}
	c.emit("alert:update", map[string]any{"id": alert.ID, "terminalStatus": status, "terminalError": detail})
}

func (c *Controller) setHealth(health core.Health) {
	if health.State == "warming" {
		health.Ready = c.engine.ReadyCount(time.Now())
		if health.Ready > 0 {
			health.State = "connected"
			health.Detail = "signals live"
		}
	}
	c.mu.Lock()
	c.health = health
	c.mu.Unlock()
	c.emit("health", health)
}

func (c *Controller) healthLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mu.RLock()
			health := c.health
			c.mu.RUnlock()
			if health.State == "warming" || health.State == "connected" {
				health.State = "warming"
				health.UpdatedAtMs = time.Now().UnixMilli()
				c.setHealth(health)
			}
		}
	}
}
