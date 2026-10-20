package core

import (
	"math"
	"sync"
	"time"
)

type ruleState struct {
	up       bool
	down     bool
	cooldown time.Time
}

type symbolState struct {
	last    time.Time
	price   float64
	bucket  int64
	windows map[int]*window
	rules   map[int64]*ruleState
}

type Engine struct {
	mu       sync.Mutex
	rules    []Rule
	settings Settings
	volumes  map[string]float64
	symbols  map[string]*symbolState
}

func NewEngine(rules []Rule, settings Settings) *Engine {
	return &Engine{
		rules: append([]Rule(nil), rules...), settings: settings,
		volumes: make(map[string]float64), symbols: make(map[string]*symbolState),
	}
}

func (e *Engine) Configure(rules []Rule, settings Settings) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = append([]Rule(nil), rules...)
	e.settings = settings
	// existing price history remains valid; each new rule keeps its own edge state.
}

func (e *Engine) SetVolumes(volumes map[string]float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for symbol, value := range volumes {
		e.volumes[symbol] = value
	}
}

func (e *Engine) Reset(symbols []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, symbol := range symbols {
		delete(e.symbols, symbol)
	}
}

func (e *Engine) ResetAll() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.symbols = make(map[string]*symbolState)
}

func (e *Engine) ReadyCount(now time.Time) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	shortest := 0
	for _, rule := range e.rules {
		if rule.Enabled && (shortest == 0 || rule.WindowSeconds < shortest) {
			shortest = rule.WindowSeconds
		}
	}
	if shortest == 0 {
		return 0
	}
	count := 0
	for symbol, state := range e.symbols {
		w := state.windows[shortest]
		if w != nil && now.Sub(w.started) >= time.Duration(shortest)*time.Second &&
			now.Sub(state.last) <= 10*time.Second && e.volumes[symbol] >= e.settings.MinQuoteVolume {
			count++
		}
	}
	return count
}

func (e *Engine) Process(q Quote) []Candidate {
	if q.Bid <= 0 || q.Ask <= 0 || q.Ask < q.Bid || q.Symbol == "" {
		return nil
	}
	if q.Received.IsZero() {
		q.Received = time.Now()
	}
	if q.EventTime.IsZero() {
		q.EventTime = q.Received
	}
	if q.Received.Sub(q.EventTime) > 10*time.Second || q.EventTime.After(q.Received.Add(2*time.Second)) {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	price := (q.Bid + q.Ask) / 2
	if math.IsInf(price, 0) || math.IsNaN(price) {
		return nil
	}
	state := e.symbols[q.Symbol]
	if state == nil || (!state.last.IsZero() && q.Received.Sub(state.last) > 10*time.Second) {
		state = &symbolState{windows: make(map[int]*window), rules: make(map[int64]*ruleState)}
		e.symbols[q.Symbol] = state
	}
	if q.Received.Before(state.last) {
		return nil
	}
	state.last = q.Received
	previous := state.price
	state.price = price
	bucket := q.Received.UnixNano() / int64(bucketSize)
	if previous == price && state.bucket == bucket {
		return nil
	}
	state.bucket = bucket

	volume := e.volumes[q.Symbol]
	var candidates []Candidate
	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}
		w := state.windows[rule.WindowSeconds]
		if w == nil {
			w = newWindow(time.Duration(rule.WindowSeconds) * time.Second)
			state.windows[rule.WindowSeconds] = w
		}
		w.add(q.Received, price)
		if q.Received.Sub(w.started) < time.Duration(rule.WindowSeconds)*time.Second {
			continue
		}
		low, high := w.bounds()
		up := (price/low - 1) * 100
		down := (price/high - 1) * 100
		previousState := state.rules[rule.ID]
		if previousState == nil {
			previousState = &ruleState{}
			state.rules[rule.ID] = previousState
		}
		upNow, downNow := up >= rule.Percent, down <= -rule.Percent
		crossUp := upNow && !previousState.up && price > previous
		crossDown := downNow && !previousState.down && price < previous
		previousState.up, previousState.down = upNow, downNow
		if volume < e.settings.MinQuoteVolume || q.Received.Before(previousState.cooldown) {
			continue
		}
		direction, change := "", 0.0
		switch {
		case crossDown:
			direction, change = "down", down
		case crossUp:
			direction, change = "up", up
		default:
			continue
		}
		previousState.cooldown = q.Received.Add(time.Duration(e.settings.CooldownSeconds) * time.Second)
		candidates = append(candidates, Candidate{
			Rule: rule, Symbol: q.Symbol, Direction: direction, ChangePercent: change,
			Price: price, SpreadPercent: (q.Ask - q.Bid) / price * 100,
			QuoteVolume: volume, EventTime: q.EventTime, Received: q.Received,
		})
	}
	return candidates
}
