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
	tiger    *Tiger
	feed     *binance.Feed
	emit     EventSink

	healthMu          sync.Mutex
	settingsMu        sync.Mutex
	mu                sync.RWMutex
	rules             []core.Rule
	settings          core.Settings
	health            core.Health
	cancel            context.CancelFunc
	stop              context.CancelFunc
	root              context.Context
	launchID          int64
	version           atomic.Uint64
	feedWG            sync.WaitGroup
	actionWG          sync.WaitGroup
	pipelineMu        sync.RWMutex
	closing           bool
	candidates        chan queuedCandidate
	notifications     chan core.Alert
	pipelineWG        sync.WaitGroup
	queueMu           sync.Mutex
	pending           map[uint64]time.Time
	sequence          atomic.Uint64
	queueHealth       core.QueueHealth
	pipelineError     error
	closeOnce         sync.Once
	closeError        error
	persistenceCtx    context.Context
	persistenceCancel context.CancelFunc
	terminalQueues    map[string]chan terminalJob
	terminalResults   chan core.AlertUpdate
	terminalWG        sync.WaitGroup
	terminalErrors    atomic.Uint64
	traceMu           sync.Mutex
	traces            map[int64]*alertTrace
	traceOrder        []int64
	traceCursor       int
	terminalWaitMs    atomic.Int64
	terminalRequestMs atomic.Int64
	renderDelayMs     atomic.Int64
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
		terminal: NewTerminal(), tiger: NewTiger(), emit: emit, rules: rules, settings: settings,
		health: core.Health{State: "starting", Detail: "initializing"},
	}
	if len(latest) > 0 {
		controller.launchID = latest[0].ID
	}
	controller.startPipeline()
	return controller, nil
}

func (c *Controller) Start(ctx context.Context) {
	c.mu.Lock()
	c.root, c.stop = context.WithCancel(ctx)
	c.mu.Unlock()
	c.feedWG.Add(1)
	go func() {
		defer c.feedWG.Done()
		if err := c.audio.Warm(); err != nil {
			slog.Warn("audio device", "error", err)
		}
	}()
	c.restartFeed()
}

func (c *Controller) Close() error {
	c.closeOnce.Do(func() { c.closeError = c.close() })
	return c.closeError
}

func (c *Controller) close() error {
	c.mu.Lock()
	if c.stop != nil {
		c.stop()
	}
	if c.cancel != nil {
		c.cancel()
	}
	c.mu.Unlock()
	c.feedWG.Wait()
	c.pipelineMu.Lock()
	c.closing = true
	close(c.candidates)
	c.pipelineMu.Unlock()
	drainTimer := time.AfterFunc(15*time.Second, c.persistenceCancel)
	defer drainTimer.Stop()
	defer c.persistenceCancel()
	c.pipelineWG.Wait()
	c.actionWG.Wait()
	for _, queue := range c.terminalQueues {
		close(queue)
	}
	close(c.terminalResults)
	c.terminalWG.Wait()
	c.tiger.Close()
	return errors.Join(c.pipelineError, c.audio.Close(), c.store.Close())
}

func (c *Controller) GetSnapshot() (core.Snapshot, error) {
	c.mu.RLock()
	snapshot := core.Snapshot{
		Rules: append([]core.Rule{}, c.rules...), Settings: c.settings, Health: c.health,
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
	feed := c.feed
	c.mu.Unlock()
	c.engine.Configure(rules, settings)
	c.emit("rules:changed", rules)
	if feed != nil {
		feed.SetMinimumVolume(core.MinimumVolume(rules))
	}
	return nil
}

func (c *Controller) SoundIDs() []string { return c.audio.IDs() }

func (c *Controller) ImportSound(name, extension, encoded string) (string, error) {
	id, err := c.audio.Import(name, extension, encoded)
	if err == nil {
		c.emit("sounds:changed", c.audio.IDs())
	}
	return id, err
}

func (c *Controller) PreviewSound(id string, volume float64) error { return c.audio.Play(id, volume) }

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

func (c *Controller) onQuote(quote core.Quote) {
	c.pipelineMu.RLock()
	defer c.pipelineMu.RUnlock()
	if c.closing {
		return
	}
	for _, candidate := range c.engine.Process(quote) {
		job := queuedCandidate{candidate: candidate, id: c.sequence.Add(1), queued: time.Now()}
		c.actionWG.Add(1)
		c.queueMu.Lock()
		c.pending[job.id] = job.queued
		select {
		case c.candidates <- job:
		default:
			delete(c.pending, job.id)
			c.queueHealth.Unsaved++
			c.queueHealth.Error = "alert storage queue overflow"
			c.pipelineError = errors.New(c.queueHealth.Error)
			c.actionWG.Done()
		}
		c.queueMu.Unlock()
	}
}
