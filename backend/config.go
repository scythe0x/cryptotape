package main

import "time"

// priceSampleRetain — как долго хранить историю тиков.
// Должен быть >= самого большого MaxDuration среди правил.
var priceSampleRetain = 5 * time.Minute

// priceImpulseRules — список правил детектирования импульса.
// Каждое правило независимо: свой порог, окно и cooldown.
// В будущем можно получать из фронтенда вместо этих констант.
var priceImpulseRules = []ImpulseRule{
	{ThresholdPct: 2.0, MaxDuration: 30 * time.Second, Cooldown: 1 * time.Second},
	// {ThresholdPct: 4.0, MaxDuration: 10 * time.Second, Cooldown: 2 * time.Minute},
}