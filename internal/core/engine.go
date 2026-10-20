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
	last     time.Time
	event    time.Time
	updateID uint64
	price    float64
	bucket   int64
	windows  map[int]*window
	rules    map[int64]*ruleState
}

func (s *symbolState) resetHistory() {
	s.last = time.Time{}
	s.event = time.Time{}
	s.updateID = 0
	s.price = 0
	s.bucket = 0
	s.windows = make(map[int]*window)
	for _, rule := range s.rules {
		rule.up, rule.down = false, false
	}
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
	previous := make(map[int64]Rule, len(e.rules))
	for _, rule := range e.rules {
		previous[rule.ID] = rule
	}
	current := make(map[int64]Rule, len(rules))
	windows := make(map[int]bool)
	for _, rule := range rules {
		current[rule.ID] = rule
		windows[rule.WindowSeconds] = true
	}
	for _, symbol := range e.symbols {
		for id := range symbol.rules {
			oldRule := previous[id]
			next, exists := current[id]
			if !exists || oldRule.Percent != next.Percent || oldRule.WindowSeconds != next.WindowSeconds {
				delete(symbol.rules, id)
			}
		}
		for seconds := range symbol.windows {
			if !windows[seconds] {
				delete(symbol.windows, seconds)
			}
		}
	}
	e.rules = append([]Rule(nil), rules...)
	e.settings = settings
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
		if state := e.symbols[symbol]; state != nil {
			state.resetHistory()
		}
	}
}

func (e *Engine) ResetAll() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, state := range e.symbols {
		state.resetHistory()
	}
}

func (e *Engine) ReadyCount(now time.Time) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	count := 0
	for symbol, state := range e.symbols {
		if now.Sub(state.last) > 10*time.Second {
			continue
		}
		for _, rule := range e.rules {
			w := state.windows[rule.WindowSeconds]
			if w != nil && e.volumes[symbol] >= rule.MinQuoteVolume {
				count++
				break
			}
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
		return nil
	}
	age := q.Received.Add(q.ClockOffset).Sub(q.EventTime)
	if age > 10*time.Second || age < -2*time.Second {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	price := (q.Bid + q.Ask) / 2
	if math.IsInf(price, 0) || math.IsNaN(price) {
		return nil
	}
	state := e.symbols[q.Symbol]
	if state == nil {
		state = &symbolState{windows: make(map[int]*window), rules: make(map[int64]*ruleState)}
		e.symbols[q.Symbol] = state
	}
	if !state.last.IsZero() && q.Received.Sub(state.last) > 10*time.Second {
		state.resetHistory()
	}
	if q.Received.Before(state.last) || q.EventTime.Before(state.event) || (q.UpdateID != 0 && state.updateID != 0 && q.UpdateID <= state.updateID) {
		return nil
	}
	state.last = q.Received
	state.event = q.EventTime
	if q.UpdateID != 0 {
		state.updateID = q.UpdateID
	}
	previous := state.price
	state.price = price
	bucket := q.EventTime.UnixNano() / int64(bucketSize)
	if previous == price && state.bucket == bucket {
		return nil
	}
	state.bucket = bucket

	volume := e.volumes[q.Symbol]
	var candidates []Candidate
	for _, rule := range e.rules {
		w := state.windows[rule.WindowSeconds]
		if w == nil {
			w = newWindow(time.Duration(rule.WindowSeconds) * time.Second)
			state.windows[rule.WindowSeconds] = w
		}
		w.add(q.EventTime, price)
		low, high := w.bounds()
		up := (price/low - 1) * 100
		down := (price/high - 1) * 100
		previousState := state.rules[rule.ID]
		if previousState == nil {
			previousState = &ruleState{}
			state.rules[rule.ID] = previousState
		}
		// tolerate floating-point error far below a tradable price increment.
		const thresholdTolerance = 1e-9
		upNow := up+thresholdTolerance >= rule.Percent
		downNow := down-thresholdTolerance <= -rule.Percent
		crossUp := upNow && !previousState.up && price > previous
		crossDown := downNow && !previousState.down && price < previous
		previousState.up, previousState.down = upNow, downNow
		if volume < rule.MinQuoteVolume || q.Received.Before(previousState.cooldown) {
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
			Detected: time.Now(), Expires: q.Received.Add(10*time.Second - age),
			Rule: rule, Symbol: q.Symbol, Direction: direction, ChangePercent: change,
			Price: price, SpreadPercent: (q.Ask - q.Bid) / price * 100,
			QuoteVolume: volume, EventTime: q.EventTime, Received: q.Received,
		})
	}
	return candidates
}

func MinimumVolume(rules []Rule) float64 {
	if len(rules) == 0 {
		return math.Inf(1)
	}
	minimum := rules[0].MinQuoteVolume
	for _, rule := range rules[1:] {
		minimum = math.Min(minimum, rule.MinQuoteVolume)
	}
	return minimum
}
