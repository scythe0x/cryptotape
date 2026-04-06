package main

import (
	"fmt"
	"time"
)

func main() {
	tickerChan := make(chan TickerUpdate, 4096)
	alertChan := make(chan Alert, 2048)

	engine := NewEngine(priceImpulseRules, priceSampleRetain)

	go StartBinanceStream(tickerChan)

	go func() {
		for ticker := range tickerChan {
			engine.Process(ticker, alertChan, time.Now())
		}
	}()

	fmt.Println("Cryptotape: импульсы цены (Binance futures !ticker@arr)")
	fmt.Println("Правила:")
	for _, r := range priceImpulseRules {
		fmt.Printf("  ±%.1f%% за %s, cooldown %s\n", r.ThresholdPct, r.MaxDuration, r.Cooldown)
	}
	fmt.Println("---")

	for alert := range alertChan {
		fmt.Printf("[%s] %s\n",
			alert.Timestamp.Format("15:04:05"),
			alert.Message,
		)
	}
}