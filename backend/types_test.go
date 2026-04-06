package main

import (
	"encoding/json"
	"testing"
)

func TestBinanceTickerLastPriceUsesLowercaseC(t *testing.T) {
	raw := `{"e":"24hrTicker","s":"WLFIUSDT","c":"0.0973000","C":1775371511170,"q":"13888692.6031","Q":"3082"}`
	var tick BinanceTicker
	if err := json.Unmarshal([]byte(raw), &tick); err != nil {
		t.Fatal(err)
	}
	if tick.Symbol != "WLFIUSDT" {
		t.Fatalf("symbol: got %q", tick.Symbol)
	}
	if string(tick.LastPrice) != "0.0973000" {
		t.Fatalf("last price must be field c, not C: got %q", tick.LastPrice)
	}
	px, err := tick.LastPrice.Float64()
	if err != nil || px < 0.09 || px > 0.1 {
		t.Fatalf("parsed price: %v err=%v", px, err)
	}
	q, err := tick.QuoteVolume.Float64()
	if err != nil || q < 1e7 {
		t.Fatalf("quote vol must be field q, not Q: got %v err=%v", tick.QuoteVolume, err)
	}
}
