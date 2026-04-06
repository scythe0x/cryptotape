package main

import (
	"fmt"
	"time"
)

// Параметры движка импульса цены. Позже часть уйдёт в API / настройки с фронта.
const (
	priceImpulseThresholdPercent = 3.0 // минимальный |Δ| в % за окно
	priceImpulseAlertCooldown    = 45 * time.Second
)

// Сколько завершённых прошлых окон той же длины усредняем для сравнения объёма (максимум — пока хватает буфера).
const priorVolumeWindowsToAverage = 24

// Глубина истории по времени: должна покрывать (priorVolumeWindowsToAverage+1) × самое длинное окно,
// иначе для длинных окон не будет ни одного «прошлого» окна в памяти (trim выкидывает якоря).
// Не привязано к алертам — сэмплы пишутся на каждом тике.
// 25×5m = 125m > 2h — для max окна 5m нужно ≥125m, берём с запасом.
var priceSampleRetain = 3 * time.Hour

const (
	impulseWindow30s = 30 * time.Second
	impulseWindow1m  = time.Minute
	impulseWindow5m  = 5 * time.Minute
)

var priceImpulseWindows = []time.Duration{
	impulseWindow1m,
}

var priceImpulseSymbolAllowlist []string

func init() {
	maxW := maxDuration(priceImpulseWindows)
	if maxW <= 0 {
		panic("priceImpulseWindows: need at least one window")
	}
	minRetain := maxW * time.Duration(priorVolumeWindowsToAverage+1)
	if priceSampleRetain < minRetain {
		panic(fmt.Sprintf(
			"priceSampleRetain (%v) must be >= %v — (priorVolumeWindowsToAverage+1)×longest window — иначе нет прошлых окон для объёма",
			priceSampleRetain, minRetain,
		))
	}
	if priceSampleRetain < maxW {
		panic("priceSampleRetain must be >= longest window")
	}
}

func maxDuration(ds []time.Duration) time.Duration {
	var m time.Duration
	for _, d := range ds {
		if d > m {
			m = d
		}
	}
	return m
}
