package main

import (
	"fmt"
	"strings"
	"time"
)

func main() {
	tickerChan := make(chan BinanceTicker, 4096)
	alertChan := make(chan Alert, 2048)
	engine := NewEngine(
		priceImpulseWindows,
		priceSampleRetain,
		priceImpulseThresholdPercent,
		priceImpulseAlertCooldown,
		priceImpulseSymbolAllowlist,
	)

	go StartBinanceStream(tickerChan)

	go func() {
		for ticker := range tickerChan {
			engine.Process(ticker, alertChan, time.Now())
		}
	}()

	fmt.Println("Cryptotape: импульсы цены (Binance futures !ticker@arr)")
	fmt.Printf(
		"Окна: %s | порог ±%.3f%% | cooldown %v\n",
		joinWindows(priceImpulseWindows),
		priceImpulseThresholdPercent,
		priceImpulseAlertCooldown,
	)
	fmt.Println("---")

	for alert := range alertChan {
		w := formatWindow(alert.Window)
		fmt.Printf("[%s] %s (%s) %s\n",
			alert.Timestamp.Format("15:04:05"),
			alert.Symbol,
			w,
			alert.Message,
		)
	}
}

func joinWindows(ds []time.Duration) string {
	parts := make([]string, len(ds))
	for i, d := range ds {
		parts[i] = formatWindow(d)
	}
	return strings.Join(parts, ", ")
}
