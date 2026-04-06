package main

import (
	"encoding/json"
	"strconv"
	"time"
)

// Binance 1s tick (update to ticker) from the !ticker@arr stream.
// https://developers.binance.com/docs/derivatives/usds-margined-futures/websocket-market-streams/All-Market-Tickers-Streams
type TickerUpdate struct {
	Symbol      string
	LastPrice   json.Number // "c"
	QuoteVolume json.Number // "q"
	TradesCount json.Number // "n"
	UpdatedAt   time.Time   // "E" (parsed from epoch ms)
}

// custom unmarshalling for Tick (binance json -> internal Tick)
func (t *TickerUpdate) UnmarshalJSON(data []byte) error {
	type raw struct {
		Symbol      string      `json:"s"`
		LastPrice   json.Number `json:"c"`
		QuoteVolume json.Number `json:"q"`
		TradesCount json.Number `json:"n"`
		UpdatedAt   json.Number `json:"E"`
	}
	var r raw
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}

	ms, err := r.UpdatedAt.Int64()
	if err != nil {
		// defensive fallback: parse string form
		ms, err = strconv.ParseInt(r.UpdatedAt.String(), 10, 64)
		if err != nil {
			return err
		}
	}

	*t = TickerUpdate{
		Symbol:      r.Symbol,
		LastPrice:   r.LastPrice,
		QuoteVolume: r.QuoteVolume,
		TradesCount: r.TradesCount,
		UpdatedAt:   time.UnixMilli(ms),
	}
	return nil
}

// Alert is sent to clients with trade signals.
type Alert struct {
	Symbol    string
	Type      string
	Message   string
	Timestamp time.Time
}
