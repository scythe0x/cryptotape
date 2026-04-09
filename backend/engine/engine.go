package engine

import (
	"strconv"
	"sync"
	"time"

	"github.com/whoiscyberial/cryptotape/backend/types"

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

	st = newSymbolState()
	e.states[symbol] = st
	return st
}

func (e *Engine) Process(tick types.BookTickerUpdate, alertChan chan<- types.Alert, now time.Time) {
	st := e.getState(tick.Symbol)

	p := pricePoint{Price: tick.MidPrice, At: now}

	st.addPoint(p)
	st.trim(now, e.maxWindow)

	out := make([]types.Alert, 0, len(e.rules))

	for i, rule := range e.rules {
		ok, pct, base := detectMove(p, st.min, st.max, rule.ThresholdPct)
		if !ok {
			continue
		}

		dir := sign(pct)
		key := tick.Symbol + "|" + strconv.Itoa(i)

		e.impMu.RLock()
		imp := e.impulses[key]
		last := e.lastBySymbol[tick.Symbol]
		e.impMu.RUnlock()

		// correction filter
		if last != nil {
			if now.Sub(last.LastAt) <= rule.MaxDuration*2 && dir != last.Direction {
				continue
			}
		}

		if isSameImpulse(imp, base, dir) {
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

		out = append(out, types.Alert{
			Symbol:     tick.Symbol,
			ImpulseID:  imp.ID,
			Type:       "PRICE_IMPULSE",
			MovePct:    (p.Price - imp.StartPrice) / imp.StartPrice * 100,
			Duration:   now.Sub(imp.StartAt),
			StartPrice: imp.StartPrice,
			EndPrice:   p.Price,
			Timestamp:  now,
		})
	}

	for _, a := range out {
		alertChan <- a
	}
}