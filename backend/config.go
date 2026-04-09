package main

import "time"

// watchSymbols — список символов для подписки на bookTicker.
// Все символы должны быть в верхнем регистре (BTCUSDT, ETHUSDT, ...).
var watchSymbols = []string{
	"STOUSDT",
	"SIRENUSDT",
	"ARIAUSDT",
	// "ETHUSDT",
	// "BNBUSDT",
	// "SOLUSDT",
	// "XRPUSDT",
}

// priceImpulseRules — правила обнаружения импульсов.
// ThresholdPct: минимальное изменение mid-price в процентах.
// MaxDuration:  временное окно, за которое должно произойти изменение.
//
// Это же окно используется для оценки динамики при повторных алертах:
// повторный алерт отправляется только если от точки последнего алерта
// цена снова прошла ThresholdPct за MaxDuration.
var priceImpulseRules = []ImpulseRule{
	{ThresholdPct: 1.4, MaxDuration: 15 * time.Second},
	// {ThresholdPct: 1.0, MaxDuration: 5 * time.Second},
}

// priceSampleRetain — как долго хранить ценовую историю.
// Должно быть >= максимального MaxDuration среди всех правил + небольшой запас.
const priceSampleRetain = 2 * time.Minute
