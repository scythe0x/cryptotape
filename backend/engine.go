package main

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// ImpulseRule описывает условие импульса:
// цена должна пройти |ThresholdPct|% за время не более MaxDuration.
type ImpulseRule struct {
	ThresholdPct float64
	MaxDuration  time.Duration
	Cooldown     time.Duration
}

type Engine struct {
	mu          sync.Mutex
	tickUpdates map[string][]TickerUpdate
	tickMaxAge  time.Duration
	rules       []ImpulseRule
	cooldown    map[string]time.Time // key: "SYMBOL|ruleIdx"
}

func NewEngine(rules []ImpulseRule, tickMaxAge time.Duration) *Engine {
	return &Engine{
		tickUpdates: make(map[string][]TickerUpdate),
		tickMaxAge:  tickMaxAge,
		rules:       rules,
		cooldown:    make(map[string]time.Time),
	}
}

func (e *Engine) appendTick(tick TickerUpdate) {
	e.tickUpdates[tick.Symbol] = append(e.tickUpdates[tick.Symbol], tick)
}

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
		e.tickUpdates[symbol] = append([]TickerUpdate{}, ticks[i:]...)
	}
}

type impulseResult struct {
	changePct float64
	basePrice float64
	elapsed   time.Duration
}

// checkImpulse проверяет один impulse rule для набора тиков.
//
// Требует минимум 2 тика — первый тик для символа всегда пропускается.
// Это намеренно: у нас нет достоверной базы до начала наблюдения,
// а поля вроде OpenPrice относятся к 24h окну и дают ложные срабатывания.
//
// Алгоритм: идём от текущего тика назад.
// Для каждого кандидата: если он вышел за MaxDuration — break.
// Если изменение >= ThresholdPct — импульс найден.
func (e *Engine) checkImpulse(ticks []TickerUpdate, rule ImpulseRule) (found bool, res impulseResult) {
	// Нужно минимум 2 тика: текущий и хотя бы одна база для сравнения.
	if len(ticks) < 2 {
		return
	}

	current := ticks[len(ticks)-1]
	currentPrice, err := current.LastPrice.Float64()
	if err != nil || currentPrice <= 0 {
		return
	}

	for i := len(ticks) - 2; i >= 0; i-- {
		candidate := ticks[i]
		dt := current.UpdatedAt.Sub(candidate.UpdatedAt)

		if dt > rule.MaxDuration {
			break // тик слишком старый, дальше только ещё старее
		}

		base, err := candidate.LastPrice.Float64()
		if err != nil || base <= 0 {
			continue
		}

		pct := (currentPrice - base) / base * 100
		if math.Abs(pct) >= rule.ThresholdPct {
			return true, impulseResult{
				changePct: pct,
				basePrice: base,
				elapsed:   dt,
			}
		}
	}

	return
}

func (e *Engine) Process(tick TickerUpdate, alertChan chan<- Alert, now time.Time) {
	price, err := tick.LastPrice.Float64()
	if err != nil || price <= 0 {
		return
	}

	e.mu.Lock()
	e.appendTick(tick)
	e.clearOldTicks(tick.Symbol, now)
	ticks := e.tickUpdates[tick.Symbol]

	var out []Alert
	for i, rule := range e.rules {
		found, res := e.checkImpulse(ticks, rule)
		if !found {
			continue
		}

		key := fmt.Sprintf("%s|%d", tick.Symbol, i)
		if last, seen := e.cooldown[key]; seen && now.Sub(last) < rule.Cooldown {
			continue
		}
		e.cooldown[key] = now

		out = append(out, Alert{
			Symbol:    tick.Symbol,
			Type:      "PRICE_IMPULSE",
			Message:   buildMessage(tick.Symbol, res, price, rule),
			Timestamp: now,
		})
	}
	e.mu.Unlock()

	for _, a := range out {
		alertChan <- a
	}
}

func buildMessage(symbol string, res impulseResult, currentPrice float64, rule ImpulseRule) string {
	direction := "▲"
	if res.changePct < 0 {
		direction = "▼"
	}
	return fmt.Sprintf(
		"%s %s %.2f%% за %s (порог: %.1f%% за %s) цена: %.4f → %.4f",
		symbol,
		direction,
		res.changePct,
		res.elapsed.Round(time.Millisecond),
		rule.ThresholdPct,
		rule.MaxDuration,
		res.basePrice,
		currentPrice,
	)
}