package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/gorilla/websocket"
)

var wsParseErrCount int

// connects to the binance websocket and writes the data to the channel
func StartBinanceStream(out chan<- TickerUpdate) {
	// https://developers.binance.com/docs/derivatives/usds-margined-futures/websocket-market-streams/All-Market-Tickers-Streams
	// all market tickers stream
	url := "wss://fstream.binance.com/ws/!ticker@arr"

	logger := log.New(os.Stdout, "[ws, binance] ", log.LstdFlags | log.Lshortfile)

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		logger.Fatal("error connecting to stream:", err)
	}
	defer conn.Close()

	logger.Println("connected to stream")

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			logger.Fatal("error reading message from stream:", err)
		}

		tickers, parsingError := parseBinanceStream(message)
		if parsingError != nil {
			logger.Fatal("parsing error: ", parsingError)
		}

		for _, t := range tickers {
			out <- t
		}
	}
}

func parseBinanceStream(raw []byte) ([]TickerUpdate, error) {
	var ticks []TickerUpdate

	if err := json.Unmarshal(raw, &ticks); err == nil {
		return ticks, nil
	}

	return ticks, nil
}
