package main

import (
	"fmt"
	"time"

	"github.com/whoiscyberial/cryptotape/backend/engine"
	"github.com/whoiscyberial/cryptotape/backend/server"
	"github.com/whoiscyberial/cryptotape/backend/types"
)

const listenAddr = ":8080"

func main() {
	tickerChan := make(chan types.BookTickerUpdate, 4096)
	alertChan := make(chan types.Alert, 2048)

	engine := engine.NewEngine(priceImpulseRules, priceSampleRetain)
	hub := server.NewHub()

	go StartBinanceStream(watchSymbols, tickerChan)

	go func() {
		for tick := range tickerChan {
			now := tick.TxTime
			if now.IsZero() {
				now = tick.EventTime
			}
			if now.IsZero() {
				now = time.Now()
			}
			engine.Process(tick, alertChan, now)
		}
	}()
	go hub.Run(alertChan)

	fmt.Println("Cryptotape: импульсы цены (Binance futures bookTicker)")
	fmt.Printf("Символы: %v\n", watchSymbols)
	fmt.Println("Правила:")
	for _, r := range priceImpulseRules {
		fmt.Printf("  ±%.1f%% за %s\n", r.ThresholdPct, r.MaxDuration)
	}

	server.StartServer(listenAddr, hub) // block
}
