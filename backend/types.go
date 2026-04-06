package main

import (
	"encoding/json"
	"strconv"
	"time"
)

// TickerUpdate — один тикер из потока !ticker@arr (Binance USDM Futures).
// Binance присылает только символы, у которых изменилось хоть одно поле.
// https://developers.binance.com/docs/derivatives/usds-margined-futures/websocket-market-streams/All-Market-Tickers-Streams
type TickerUpdate struct {
	Symbol      string
	LastPrice   json.Number // "c" — последняя цена сделки
	PriceChange json.Number // "p" — абсолютное изменение цены за rolling 24h
	PriceChangePct json.Number // "P" — % изменения за rolling 24h
	OpenPrice   json.Number // "o" — цена открытия rolling 24h окна
	HighPrice   json.Number // "h" — максимум за 24h
	LowPrice    json.Number // "l" — минимум за 24h
	WeightedAvgPrice json.Number // "w" — средневзвешенная цена
	BaseVolume  json.Number // "v" — объём базового актива
	QuoteVolume json.Number // "q" — объём котируемого актива
	LastQty     json.Number // "Q" — объём последней сделки
	TradesCount json.Number // "n" — количество сделок
	OpenTime    time.Time   // "O" — начало rolling окна
	CloseTime   time.Time   // "C" — конец rolling окна
	FirstTradeID int64      // "F"
	LastTradeID  int64      // "L"
	UpdatedAt   time.Time   // "E" — время события (используется как timestamp тика)
}

func (t *TickerUpdate) UnmarshalJSON(data []byte) error {
	// Явно объявляем "e" как string — иначе Go json матчит "e" и "E"
	// без учёта регистра (case-insensitive fallback), и "24hrTicker"
	// попадает в поле UpdatedAt типа json.Number → ошибка парсинга.
	type raw struct {
		EventType        string      `json:"e"` // "24hrTicker" — нужно объявить явно
		Symbol           string      `json:"s"`
		LastPrice        json.Number `json:"c"`
		PriceChange      json.Number `json:"p"`
		PriceChangePct   json.Number `json:"P"`
		OpenPrice        json.Number `json:"o"`
		HighPrice        json.Number `json:"h"`
		LowPrice         json.Number `json:"l"`
		WeightedAvgPrice json.Number `json:"w"`
		BaseVolume       json.Number `json:"v"`
		QuoteVolume      json.Number `json:"q"`
		LastQty          json.Number `json:"Q"`
		TradesCount      json.Number `json:"n"`
		OpenTime         json.Number `json:"O"`
		CloseTime        json.Number `json:"C"`
		FirstTradeID     int64       `json:"F"`
		LastTradeID      int64       `json:"L"`
		UpdatedAt        json.Number `json:"E"`
	}
	var r raw
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}

	updatedAt, err := parseEpochMs(r.UpdatedAt)
	if err != nil {
		return err
	}
	openTime, _ := parseEpochMs(r.OpenTime)   // не критично если не распарсится
	closeTime, _ := parseEpochMs(r.CloseTime)

	*t = TickerUpdate{
		Symbol:           r.Symbol,
		LastPrice:        r.LastPrice,
		PriceChange:      r.PriceChange,
		PriceChangePct:   r.PriceChangePct,
		OpenPrice:        r.OpenPrice,
		HighPrice:        r.HighPrice,
		LowPrice:         r.LowPrice,
		WeightedAvgPrice: r.WeightedAvgPrice,
		BaseVolume:       r.BaseVolume,
		QuoteVolume:      r.QuoteVolume,
		LastQty:          r.LastQty,
		TradesCount:      r.TradesCount,
		OpenTime:         openTime,
		CloseTime:        closeTime,
		FirstTradeID:     r.FirstTradeID,
		LastTradeID:      r.LastTradeID,
		UpdatedAt:        updatedAt,
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

// Alert — сигнал, отправляемый клиентам.
type Alert struct {
	Symbol    string
	Type      string
	Message   string
	Timestamp time.Time
}