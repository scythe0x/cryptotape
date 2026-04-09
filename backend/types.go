package main

import (
	"encoding/json"
	"strconv"
	"time"
)

// BookTickerUpdate — обновление лучшего bid/ask из потока <symbol>@bookTicker.
// https://developers.binance.com/docs/derivatives/usds-margined-futures/websocket-market-streams/Individual-Symbol-Book-Ticker-Streams
type BookTickerUpdate struct {
	Symbol    string
	UpdateID  int64
	EventTime time.Time
	TxTime    time.Time
	BidPrice  float64 // "b" — лучшая цена покупки
	BidQty    float64 // "B" — объём на лучшем bid
	AskPrice  float64 // "a" — лучшая цена продажи
	AskQty    float64 // "A" — объём на лучшем ask
	MidPrice  float64 // (bid+ask)/2
}

// combinedWrapper — обёртка combined stream Binance: {"stream":"...","data":{...}}
type combinedWrapper struct {
	Stream string          `json:"stream"`
	Data   json.RawMessage `json:"data"`
}

func (t *BookTickerUpdate) UnmarshalJSON(data []byte) error {
	type raw struct {
		EventType string      `json:"e"`
		UpdateID  int64       `json:"u"`
		EventTime json.Number `json:"E"`
		TxTime    json.Number `json:"T"`
		Symbol    string      `json:"s"`
		BidPrice  json.Number `json:"b"`
		BidQty    json.Number `json:"B"`
		AskPrice  json.Number `json:"a"`
		AskQty    json.Number `json:"A"`
	}

	var r raw
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}

	eventTime, err := parseEpochMs(r.EventTime)
	if err != nil {
		return err
	}

	txTime, _ := parseEpochMs(r.TxTime)

	bid, _ := r.BidPrice.Float64()
	ask, _ := r.AskPrice.Float64()
	bidQty, _ := r.BidQty.Float64()
	askQty, _ := r.AskQty.Float64()

	mid := (bid + ask) / 2

	*t = BookTickerUpdate{
		Symbol:    r.Symbol,
		UpdateID:  r.UpdateID,
		EventTime: eventTime,
		TxTime:    txTime,
		BidPrice:  bid,
		BidQty:    bidQty,
		AskPrice:  ask,
		AskQty:    askQty,
		MidPrice:  mid,
	}

	return nil
}

func parseEpochMs(n json.Number) (time.Time, error) {
	ms, err := n.Int64()
	if err != nil {
		ms, err = strconv.ParseInt(n.String(), 10, 64)
		if err != nil {
			return time.Time{}, err
		}
	}
	return time.UnixMilli(ms), nil
}

// Alert — сигнал об импульсе цены.
type Alert struct {
	Symbol     string
	ImpulseID  string
	Type       string
	MovePct    float64
	Duration   time.Duration
	StartPrice float64
	EndPrice   float64
	Timestamp  time.Time
}