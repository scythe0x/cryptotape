package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

const (
	binanceBaseURL = "wss://fstream.binance.com/stream?streams="
)

// main function - builds an url with buildCombinedURL() and connects to WS with rubBinanceConn()
func StartBinanceStream(symbols []string, out chan<- BookTickerUpdate) {
	logger := log.New(os.Stdout, "[ws/binance] ", log.LstdFlags|log.Lshortfile)

	url := buildCombinedURL(symbols)
	logger.Printf("stream URL: %s", url)

	err := runBinanceConn(url, out, logger)
	if err != nil {
		logger.Printf("connection error: %v", err)
	}
}

// generates stream URL from watchlist
// e.g.: wss://fstream.binance.com/stream?streams=btcusdt@bookTicker/ethusdt@bookTicker
func buildCombinedURL(symbols []string) string {
	streams := make([]string, len(symbols))
	for i, s := range symbols {
		streams[i] = strings.ToLower(s) + "@bookTicker"
	}
	return binanceBaseURL + strings.Join(streams, "/")
}

// connects to binance stream websocket and streams parsed messages to a channel
func runBinanceConn(url string, out chan<- BookTickerUpdate, logger *log.Logger) error {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	logger.Println("connected to binance WS")

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		tick, err := parseCombinedBookTicker(message)
		if err != nil {
			logger.Fatalf("parse error: %v", err)
		}

		out <- *tick
	}
}

// parse WS message and convert it to BookTickerUpdate
func parseCombinedBookTicker(raw []byte) (*BookTickerUpdate, error) {
	var wrapper combinedWrapper
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, fmt.Errorf("wrapper: %w", err)
	}

	var tick BookTickerUpdate
	if err := json.Unmarshal(wrapper.Data, &tick); err != nil {
		return nil, fmt.Errorf("bookTicker data: %w", err)
	}

	return &tick, nil
}
