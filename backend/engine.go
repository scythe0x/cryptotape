package main

import (
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ImpulseRule struct {
	ThresholdPct float64
	MaxDuration  time.Duration
}

type pricePoint struct {
	Price float64
	At    time.Time
}

type impulse struct {
	ID         string
	Direction  int
	StartPrice float64
	StartAt    time.Time
	LastPrice  float64
	LastAt     time.Time
}

type symbolState struct {
	mu sync.Mutex

	points []pricePoint // sliding window of recent ticks
	min    pricePoint   // min price in window
	max    pricePoint   // max price in window
}

type Engine struct {
	statesMu sync.RWMutex
	states   map[string]*symbolState

	rules []ImpulseRule

	impMu        sync.RWMutex
	impulses     map[string]*impulse
	lastBySymbol map[string]*impulse

	maxWindow time.Duration
}

func NewEngine(rules []ImpulseRule, _ time.Duration) *Engine {
	var maxW time.Duration
	for _, r := range rules {
		if r.MaxDuration > maxW {
			maxW = r.MaxDuration
		}
	}

	return &Engine{
		states:       make(map[string]*symbolState),
		rules:        rules,
		impulses:     make(map[string]*impulse),
		lastBySymbol: make(map[string]*impulse),
		maxWindow:    maxW,
	}
}

func sign(pct float64) int {
	if pct >= 0 {
		return 1
	}
	return -1
}

// get or create state for symbol (double-check locking)
func (e *Engine) getState(symbol string) *symbolState {
	e.statesMu.RLock()
	st, ok := e.states[symbol]
	e.statesMu.RUnlock()
	if ok {
		return st
	}

	e.statesMu.Lock()
	defer e.statesMu.Unlock()

	if st, ok = e.states[symbol]; ok {
		return st
	}

	st = &symbolState{
		points: make([]pricePoint, 0, 128), // prealloc small buffer
	}
	e.states[symbol] = st
	return st
}

// trim old points without extra allocations
func (e *Engine) trim(st *symbolState, now time.Time) {
	cutoff := now.Add(-e.maxWindow)

	i := 0
	for i < len(st.points) && st.points[i].At.Before(cutoff) {
		i++
	}

	if i > 0 {
		// reuse underlying array (no copy)
		st.points = st.points[i:]
	}

	if len(st.points) == 0 {
		return
	}

	// recalc min/max only if needed
	if st.min.At.Before(cutoff) || st.max.At.Before(cutoff) {
		min := st.points[0]
		max := st.points[0]

		for _, p := range st.points {
			if p.Price < min.Price {
				min = p
			}
			if p.Price > max.Price {
				max = p
			}
		}

		st.min = min
		st.max = max
	}
}

func (e *Engine) Process(tick BookTickerUpdate, alertChan chan<- Alert, now time.Time) {
	st := e.getState(tick.Symbol)

	st.mu.Lock()

	p := pricePoint{Price: tick.MidPrice, At: now}
	st.points = append(st.points, p)

	// init min/max
	if len(st.points) == 1 {
		st.min = p
		st.max = p
	}

	e.trim(st, now)

	// update extrema
	if p.Price < st.min.Price {
		st.min = p
	}
	if p.Price > st.max.Price {
		st.max = p
	}

	out := make([]Alert, 0, len(e.rules))

	for i, rule := range e.rules {
		upMove := (p.Price - st.min.Price) / st.min.Price * 100
		downMove := (p.Price - st.max.Price) / st.max.Price * 100

		var detPct float64
		var base pricePoint
		var ok bool

		if upMove >= rule.ThresholdPct {
			detPct = upMove
			base = st.min
			ok = true
		} else if downMove <= -rule.ThresholdPct {
			detPct = downMove
			base = st.max
			ok = true
		}

		if !ok {
			continue
		}

		dir := sign(detPct)

		key := tick.Symbol + "|" + strconv.Itoa(i) // faster than Sprintf

		e.impMu.RLock()
		imp := e.impulses[key]
		last := e.lastBySymbol[tick.Symbol]
		e.impMu.RUnlock()

		// ignore opposite impulse inside same window (correction)
		if last != nil {
			if now.Sub(last.LastAt) <= rule.MaxDuration*2 && dir != last.Direction {
				continue
			}
		}

		sameImpulse := imp != nil &&
			base.At.Before(imp.LastAt) &&
			imp.Direction == dir

		if sameImpulse {
			// only extend if new extreme
			if sign(p.Price-imp.LastPrice) != dir {
				continue
			}

			e.impMu.Lock()
			imp.LastPrice = p.Price
			imp.LastAt = now
			e.impMu.Unlock()
		} else {
			newImp := &impulse{
				ID:         tick.Symbol + "|" + uuid.New().String()[:8],
				Direction:  dir,
				StartPrice: base.Price,
				StartAt:    base.At,
				LastPrice:  p.Price,
				LastAt:     now,
			}

			e.impMu.Lock()
			e.impulses[key] = newImp
			e.lastBySymbol[tick.Symbol] = newImp
			e.impMu.Unlock()

			imp = newImp
		}

		totalPct := (p.Price - imp.StartPrice) / imp.StartPrice * 100
		totalElapsed := now.Sub(imp.StartAt)

		arrow := "▲"
		if totalPct < 0 {
			arrow = "▼"
		}

		// build message without fmt.Sprintf (less allocations)
		msg := tick.Symbol + " " + arrow + " " +
			strconv.FormatFloat(totalPct, 'f', 2, 64) + "% за " +
			totalElapsed.Round(time.Millisecond).String() +
			" mid: " +
			strconv.FormatFloat(imp.StartPrice, 'f', 6, 64) +
			" → " +
			strconv.FormatFloat(p.Price, 'f', 6, 64)

		out = append(out, Alert{
			Symbol:    tick.Symbol,
			ImpulseID: imp.ID,
			Type:      "PRICE_IMPULSE",
			Message:   msg,
			Timestamp: now,
		})
	}

	st.mu.Unlock()

	for _, a := range out {
		alertChan <- a
	}
}