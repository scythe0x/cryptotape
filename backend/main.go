package main

import (
	"fmt"
	"time"

	"github.com/whoiscyberial/cryptotape/backend/engine"
	"github.com/whoiscyberial/cryptotape/backend/types"
)

func main() {
	tickerChan := make(chan types.BookTickerUpdate, 4096)
	alertChan := make(chan types.Alert, 2048)

	engine := engine.NewEngine(priceImpulseRules, priceSampleRetain)

	go StartBinanceStream(watchSymbols, tickerChan)

	go func() {
		for tick := range tickerChan {
			engine.Process(tick, alertChan, time.Now())
		}
	}()

	fmt.Println("Cryptotape: импульсы цены (Binance futures bookTicker)")
	fmt.Printf("Символы: %v\n", watchSymbols)
	fmt.Println("Правила:")
	for _, r := range priceImpulseRules {
		fmt.Printf("  ±%.1f%% за %s\n", r.ThresholdPct, r.MaxDuration)
	}
	fmt.Println("---")

	for alert := range alertChan {
		arrow := "▲"
		if alert.MovePct < 0 {
			arrow = "▼"
		}
	
		fmt.Printf("[%s] [%s] %s %s %.2f%% за %s mid: %.6f → %.6f\n",
			alert.Timestamp.Format("15:04:05"),
			alert.ImpulseID,
			alert.Symbol,
			arrow,
			alert.MovePct,
			alert.Duration.Round(time.Millisecond),
			alert.StartPrice,
			alert.EndPrice,
		)
	}
}
