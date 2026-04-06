package main

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

const (
	binanceURL     = "wss://fstream.binance.com/ws/!ticker@arr"
	reconnectDelay = 5 * time.Second
)

// StartBinanceStream подключается к Binance !ticker@arr и пишет тики в out.
// При обрыве соединения переподключается через reconnectDelay.
// Завершается только если out закрыт (никогда при нормальной работе).
func StartBinanceStream(out chan<- TickerUpdate) {
	logger := log.New(os.Stdout, "[ws/binance] ", log.LstdFlags|log.Lshortfile)

	for {
		if err := runBinanceConn(out, logger); err != nil {
			logger.Printf("connection error: %v — reconnecting in %s", err, reconnectDelay)
		}
		time.Sleep(reconnectDelay)
	}
}

func runBinanceConn(out chan<- TickerUpdate, logger *log.Logger) error {
	conn, _, err := websocket.DefaultDialer.Dial(binanceURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	logger.Println("connected")

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		tickers, err := parseBinanceStream(message)
		if err != nil {
			// Одно битое сообщение не должно убивать поток
			logger.Printf("parse error (skipping): %v", err)
			continue
		}

		for _, t := range tickers {
			out <- t
		}
	}
}

func parseBinanceStream(raw []byte) ([]TickerUpdate, error) {
	var ticks []TickerUpdate
	if err := json.Unmarshal(raw, &ticks); err != nil {
		return nil, err
	}
	return ticks, nil
}