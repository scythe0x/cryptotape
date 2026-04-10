package types

import (
	"encoding/json"
	"strconv"
	"time"
)

type BookTickerUpdate struct {
	Symbol    string
	UpdateID  int64
	EventTime time.Time
	TxTime    time.Time
	BidPrice  float64
	BidQty    float64
	AskPrice  float64
	AskQty    float64
	MidPrice  float64 // (bid+ask)/2
}

type CombinedWrapper struct {
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
	*t = BookTickerUpdate{
		Symbol:    r.Symbol,
		UpdateID:  r.UpdateID,
		EventTime: eventTime,
		TxTime:    txTime,
		BidPrice:  bid,
		BidQty:    bidQty,
		AskPrice:  ask,
		AskQty:    askQty,
		MidPrice:  (bid + ask) / 2,
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

// Alert — импульс цены. Фронт использует ImpulseID для upsert в ленте событий.
type Alert struct {
	ImpulseID  string    `json:"impulseId"`
	Symbol     string    `json:"symbol"`
	Direction  string    `json:"direction"`  // "up" | "down"
	CurrentPct float64   `json:"currentPct"` // текущий % от начала импульса
	MaxPtc     float64   `json:"maxPtc"`     // максимальный пройденный % от начала импульса
	DurationMs int64     `json:"durationMs"` // длительность импульса в миллисекундах
	StartPrice float64   `json:"startPrice"`
	EndPrice   float64   `json:"endPrice"`
	Timestamp  time.Time `json:"timestamp"`
}
