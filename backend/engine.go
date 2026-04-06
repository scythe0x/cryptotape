package main

import (
	"sync"
	"time"
)

type Engine struct {
	mu        sync.Mutex
	tickUpdates map[string][]TickerUpdate // BTC: [tick1, tick2, ...] -- stores all changes to ticker.
	tickMaxAge time.Duration
}

func NewEngine(ticks []TickerUpdate, tickMaxAge time.Duration) *Engine {
	return &Engine{
		tickUpdates: make(map[string][]TickerUpdate),
		tickMaxAge: tickMaxAge,
	}
}

// add new tick to history
func (e *Engine) appendTick(tick TickerUpdate) {
	initialTicks := e.tickUpdates[tick.Symbol]
	e.tickUpdates[tick.Symbol] = append(initialTicks, tick)
}

// remove old ticks that will neved be used
func (e *Engine) clearOldTicks(symbol string, now time.Time) {
	ticks := e.tickUpdates[symbol]
	if len(ticks) == 0 {
		return
	}
	cutoff := now.Add(-e.tickMaxAge)

	i := 0
	for i < len(ticks) && ticks[i].UpdatedAt.Before(cutoff) {
		i++
	}
	if i > 0 {
		copy(ticks, ticks[i:])
		e.tickUpdates[symbol] = ticks[:len(ticks)-i]
	}
}


func (e *Engine) Process(tick TickerUpdate, alertChan chan<- Alert, now time.Time) {
	// price, err := tick.LastPrice.Float64()
	// if err != nil || price <= 0 { return }

	// quoteVolume, err := tick.QuoteVolume.Float64()
	// if err != nil { return }

	e.mu.Lock()
	e.appendTick(tick)
	e.clearOldTicks(tick.Symbol, now)

	var out []Alert
	// for _, w := range e.windows {
	// 	base, ok := e.priceAtOrBefore(ticker.Symbol, now.Add(-w))
	// 	if !ok || base <= 0 { continue }
	// 	changePct := (price - base) / base * 100
	// 	if changePct > -e.threshold && changePct < e.threshold { continue }

	// 	key := ticker.Symbol + "|" + w.String()
	// 	if last, seen := e.cooldown[key]; seen && now.Sub(last) < e.cooldownD { continue }
	// 	e.cooldown[key] = now

	// 	msg := e.buildPriceImpulseMessage(ticker.Symbol, now, w, changePct, price, base, qOK)
	// 	out = append(out, Alert{Symbol: ticker.Symbol, Type: "PRICE_IMPULSE", Window: w, Message: msg, Timestamp: now})
	// }
	e.mu.Unlock()

	for _, a := range out { alertChan <- a }
}