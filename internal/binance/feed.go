package binance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"cryptotape/internal/core"
)

const (
	restBase  = "https://fapi.binance.com"
	wsPublic  = "wss://fstream.binance.com/public/stream"
	shardSize = 200
)

type Callbacks struct {
	Quote  func(core.Quote)
	Volume func(map[string]float64)
	Gap    func([]string)
	Health func(core.Health)
	Ping   func(time.Duration)
}

type Feed struct {
	client    *http.Client
	minVolume float64
	cb        Callbacks

	mu            sync.Mutex
	shards        int
	symbols       []string
	known         map[string]bool
	volumes       map[string]float64
	eligible      map[string]bool
	shardMembers  [][]string
	symbolShard   map[string]int
	updates       []chan struct{}
	clockOffset   atomic.Int64
	streams       map[int]*streamState
	volumeUpdated time.Time
	serviceErrors map[string]string
	usedWeight    int64
	rest          *restGate
	startShard    func(int)
	rotationAfter time.Duration
}

func NewFeed(proxy string, minVolume float64, cb Callbacks) (*Feed, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	route := "direct"
	if proxy != "" {
		address := proxy
		if !strings.Contains(address, "://") {
			address = "http://" + address
		}
		proxyURL, err := url.Parse(address)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(proxyURL)
		route = proxyURL.Scheme + "://" + strings.ToLower(proxyURL.Host)
	}
	return &Feed{rest: gateForRoute(route), minVolume: minVolume, client: &http.Client{Transport: transport, Timeout: 8 * time.Second}, cb: cb}, nil
}

func (f *Feed) Test(ctx context.Context) error {
	var result struct {
		ServerTime int64 `json:"serverTime"`
	}
	if err := f.get(ctx, "/fapi/v1/time", &result); err != nil {
		return err
	}
	if result.ServerTime == 0 {
		return errors.New("binance returned no server time")
	}
	connection, _, err := websocket.Dial(ctx, wsPublic+"?streams=btcusdt@bookTicker", &websocket.DialOptions{HTTPClient: f.client})
	if err != nil {
		return fmt.Errorf("binance websocket: %w", err)
	}
	defer connection.CloseNow()
	connection.SetReadLimit(8 << 20)
	_, raw, err := connection.Read(ctx)
	if err != nil {
		return err
	}
	var frame bookFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return err
	}
	if frame.Data.Symbol != "BTCUSDT" || (frame.Data.Event == 0 && frame.Data.Trade == 0) {
		return errors.New("binance returned no timestamped book quote")
	}
	return nil
}

func (f *Feed) Run(ctx context.Context) error {
	defer f.client.CloseIdleConnections()
	f.status("connecting", "loading symbols and turnover")
	if err := f.syncClock(ctx); err != nil {
		return err
	}
	symbols, err := f.loadSymbols(ctx)
	if err != nil {
		return err
	}
	volumes, err := f.loadVolumes(ctx)
	if err != nil {
		return err
	}
	f.cb.Volume(volumes)
	f.mu.Lock()
	f.symbols = symbols
	f.known = make(map[string]bool, len(symbols))
	f.volumes = volumes
	f.eligible = make(map[string]bool)
	f.symbolShard = make(map[string]int, len(symbols))
	f.streams = make(map[int]*streamState)
	f.serviceErrors = make(map[string]string)
	f.volumeUpdated = time.Now()
	for _, symbol := range symbols {
		f.known[symbol] = true
		shard := len(f.shardMembers) - 1
		if shard < 0 || len(f.shardMembers[shard]) >= shardSize {
			f.shardMembers = append(f.shardMembers, nil)
			f.updates = append(f.updates, make(chan struct{}, 1))
			shard++
		}
		f.shardMembers[shard] = append(f.shardMembers[shard], symbol)
		f.symbolShard[symbol] = shard
		f.eligible[symbol] = volumes[symbol] >= f.minVolume
	}
	f.shards = len(f.shardMembers)
	var wg sync.WaitGroup
	f.startShard = func(shard int) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := f.bookLoop(ctx, shard)
			if ctx.Err() != nil {
				return
			}
			detail := "price stream stopped"
			if err != nil {
				detail = err.Error()
			}
			f.setStream(shard, func(s *streamState) { s.health.State = "error"; s.health.Detail = detail; s.health.Subscriptions = 0 })
			f.status("warming", "signals live")
		}()
	}
	initial := f.shards
	f.mu.Unlock()
	for shard := range initial {
		f.startShard(shard)
	}
	wg.Add(2)
	go func() { defer wg.Done(); f.refreshLoop(ctx) }()
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				f.status("warming", "signals live")
			}
		}
	}()
	wg.Wait()
	return context.Cause(ctx)
}

func (f *Feed) syncClock(ctx context.Context) error {
	var result struct {
		ServerTime int64 `json:"serverTime"`
	}
	start := time.Now()
	var timingMu sync.Mutex
	trace := &httptrace.ClientTrace{GotConn: func(httptrace.GotConnInfo) {
		// exclude rest admission, proxy negotiation and TLS from the clock sample.
		timingMu.Lock()
		start = time.Now()
		timingMu.Unlock()
	}}
	if err := f.get(httptrace.WithClientTrace(ctx, trace), "/fapi/v1/time", &result); err != nil {
		return err
	}
	end := time.Now()
	if result.ServerTime == 0 {
		return errors.New("invalid Binance clock")
	}
	timingMu.Lock()
	localMidpoint := start.Add(end.Sub(start) / 2)
	timingMu.Unlock()
	f.clockOffset.Store(time.UnixMilli(result.ServerTime).Sub(localMidpoint).Nanoseconds())
	return nil
}

func (f *Feed) loadSymbols(ctx context.Context) ([]string, error) {
	var exchange struct {
		Symbols []struct {
			Symbol     string `json:"symbol"`
			Status     string `json:"status"`
			QuoteAsset string `json:"quoteAsset"`
		} `json:"symbols"`
	}
	if err := f.get(ctx, "/fapi/v1/exchangeInfo", &exchange); err != nil {
		return nil, err
	}
	symbols := make([]string, 0, len(exchange.Symbols))
	for _, market := range exchange.Symbols {
		if market.Status == "TRADING" && market.QuoteAsset == "USDT" {
			symbols = append(symbols, market.Symbol)
		}
	}
	sort.Strings(symbols)
	if len(symbols) == 0 {
		return nil, errors.New("no trading USDT futures returned by Binance")
	}
	return symbols, nil
}

func (f *Feed) loadVolumes(ctx context.Context) (map[string]float64, error) {
	var tickers []struct {
		Symbol string `json:"symbol"`
		Quote  string `json:"quoteVolume"`
	}
	if err := f.get(ctx, "/fapi/v1/ticker/24hr", &tickers); err != nil {
		return nil, err
	}
	volumes := make(map[string]float64, len(tickers))
	for _, ticker := range tickers {
		value, err := strconv.ParseFloat(ticker.Quote, 64)
		if err == nil {
			volumes[ticker.Symbol] = value
		}
	}
	return volumes, nil
}

func (f *Feed) updateEligibility(values map[string]float64) {
	changed := make(map[int]bool)
	var added []string
	f.mu.Lock()
	for symbol, volume := range values {
		// turnover can refresh before exchangeInfo discovers a new listing.
		f.volumes[symbol] = volume
		if !f.known[symbol] {
			continue
		}
		wanted := volume >= f.minVolume
		if f.eligible[symbol] == wanted {
			continue
		}
		f.eligible[symbol] = wanted
		if wanted {
			added = append(added, symbol)
		}
		changed[f.symbolShard[symbol]] = true
	}
	updates := append([]chan struct{}(nil), f.updates...)
	f.mu.Unlock()
	if len(added) > 0 && f.cb.Gap != nil {
		f.cb.Gap(added)
	}
	for shard := range changed {
		select {
		case updates[shard] <- struct{}{}:
		default:
		}
	}
}

func (f *Feed) desiredStreams(shard int) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	streams := make([]string, 0, len(f.shardMembers[shard]))
	for _, symbol := range f.shardMembers[shard] {
		if f.eligible[symbol] {
			streams = append(streams, strings.ToLower(symbol)+"@bookTicker")
		}
	}
	return streams
}

func websocketWriteJSON(ctx context.Context, connection *websocket.Conn, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return connection.Write(ctx, websocket.MessageText, data)
}

func writeSubscription(ctx context.Context, connection *websocket.Conn, method string, streams []string, id int) error {
	if len(streams) == 0 {
		return nil
	}
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	request := struct {
		Method string   `json:"method"`
		Params []string `json:"params"`
		ID     int      `json:"id"`
	}{Method: method, Params: streams, ID: id}
	return websocketWriteJSON(writeCtx, connection, request)
}

func pingUntilDone(ctx context.Context, connection *websocket.Conn, report func(time.Duration)) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		started := time.Now()
		err := connection.Ping(pingCtx)
		elapsed := time.Since(started)
		cancel()
		if err != nil {
			connection.CloseNow()
			return
		}
		if report != nil {
			report(elapsed)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func pause(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (f *Feed) SetMinimumVolume(minimum float64) {
	f.mu.Lock()
	f.minVolume = minimum
	changed := make(map[int]bool)
	var added []string
	for symbol, volume := range f.volumes {
		if !f.known[symbol] {
			continue
		}
		wanted := volume >= minimum
		if f.eligible[symbol] != wanted {
			f.eligible[symbol] = wanted
			if wanted {
				added = append(added, symbol)
			}
			changed[f.symbolShard[symbol]] = true
		}
	}
	updates := append([]chan struct{}(nil), f.updates...)
	f.mu.Unlock()
	if len(added) > 0 && f.cb.Gap != nil {
		f.cb.Gap(added)
	}
	for shard := range changed {
		select {
		case updates[shard] <- struct{}{}:
		default:
		}
	}
}

func (f *Feed) refreshSymbols(symbols []string) {
	f.mu.Lock()
	next := make(map[string]bool, len(symbols))
	for _, symbol := range symbols {
		next[symbol] = true
	}
	var removed []string
	for symbol := range f.known {
		if !next[symbol] {
			removed = append(removed, symbol)
			delete(f.volumes, symbol)
			delete(f.eligible, symbol)
		}
	}
	for shard, members := range f.shardMembers {
		kept := make([]string, 0, len(members))
		for _, symbol := range members {
			if next[symbol] {
				kept = append(kept, symbol)
			}
		}
		f.shardMembers[shard] = kept
	}
	var addedShards []int
	for _, symbol := range symbols {
		if f.known[symbol] {
			continue
		}
		target := -1
		for shard, members := range f.shardMembers {
			if len(members) < shardSize && (target < 0 || len(members) < len(f.shardMembers[target])) {
				target = shard
			}
		}
		if target < 0 {
			target = len(f.shardMembers)
			f.shardMembers = append(f.shardMembers, nil)
			f.updates = append(f.updates, make(chan struct{}, 1))
			addedShards = append(addedShards, target)
		}
		f.shardMembers[target] = append(f.shardMembers[target], symbol)
		f.eligible[symbol] = f.volumes[symbol] >= f.minVolume
	}
	f.shards = len(f.shardMembers)
	f.symbolShard = make(map[string]int, len(symbols))
	for shard, members := range f.shardMembers {
		for _, symbol := range members {
			f.symbolShard[symbol] = shard
		}
	}
	f.known = next
	f.symbols = append([]string(nil), symbols...)
	updates := append([]chan struct{}(nil), f.updates...)
	start := f.startShard
	f.mu.Unlock()
	if len(removed) > 0 {
		f.cb.Gap(removed)
	}
	for _, shard := range addedShards {
		if start != nil {
			start(shard)
		}
	}
	for _, update := range updates {
		select {
		case update <- struct{}{}:
		default:
		}
	}
}
