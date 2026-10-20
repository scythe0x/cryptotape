package binance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"slices"
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
	wsMarket  = "wss://fstream.binance.com/market/stream"
	shardSize = 200
)

type Callbacks struct {
	Quote  func(core.Quote)
	Volume func(map[string]float64)
	Gap    func([]string)
	Health func(core.Health)
}

type Feed struct {
	client    *http.Client
	minVolume float64
	cb        Callbacks

	mu           sync.Mutex
	active       int
	shards       int
	symbols      []string
	known        map[string]bool
	volumes      map[string]float64
	eligible     map[string]bool
	shardMembers [][]string
	updates      []chan struct{}
	volumeOK     atomic.Bool
	clockOffset  atomic.Int64
	volumeReady  chan struct{}
	volumeOnce   sync.Once
}

func NewFeed(proxy string, minVolume float64, cb Callbacks) (*Feed, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	if proxy != "" {
		proxyURL, err := url.Parse("http://" + proxy)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	return &Feed{minVolume: minVolume, client: &http.Client{Transport: transport}, cb: cb}, nil
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
	connection, _, err := websocket.Dial(ctx, wsMarket+"?streams=!miniTicker@arr", &websocket.DialOptions{HTTPClient: f.client})
	if err != nil {
		return fmt.Errorf("binance websocket: %w", err)
	}
	defer connection.CloseNow()
	connection.SetReadLimit(8 << 20)
	_, _, err = connection.Read(ctx)
	return err
}

func (f *Feed) Run(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	f.status("connecting", "loading symbols and turnover")
	if err := f.syncClock(runCtx); err != nil {
		f.status("error", err.Error())
		return err
	}
	symbols, err := f.loadSymbols(runCtx)
	if err != nil {
		f.status("error", err.Error())
		return err
	}
	volumes, err := f.loadVolumes(runCtx)
	if err != nil {
		f.status("error", err.Error())
		return err
	}
	f.cb.Volume(volumes)
	f.mu.Lock()
	f.symbols = symbols
	f.shards = (len(symbols) + shardSize - 1) / shardSize
	f.known = make(map[string]bool, len(symbols))
	f.volumes = volumes
	f.eligible = make(map[string]bool)
	f.shardMembers = make([][]string, f.shards)
	f.updates = make([]chan struct{}, f.shards)
	f.volumeReady = make(chan struct{})
	for shard := range f.updates {
		f.updates[shard] = make(chan struct{}, 1)
	}
	for index, symbol := range symbols {
		f.known[symbol] = true
		f.shardMembers[index/shardSize] = append(f.shardMembers[index/shardSize], symbol)
		if volumes[symbol] >= f.minVolume {
			f.eligible[symbol] = true
		}
	}
	f.mu.Unlock()
	f.status("connecting", "opening market streams")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		f.volumeLoop(runCtx)
	}()
	select {
	case <-f.volumeReady:
	case <-runCtx.Done():
		wg.Wait()
		return runCtx.Err()
	}
	for shard := 0; shard < f.shards; shard++ {
		wg.Add(1)
		go func(shard int) {
			defer wg.Done()
			f.bookLoop(runCtx, shard)
		}(shard)
	}
	changes := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				if err := f.syncClock(runCtx); err != nil {
					f.status("degraded", "clock sync: "+err.Error())
					continue
				}
				current, err := f.loadSymbols(runCtx)
				if err != nil {
					f.status("degraded", "symbol refresh: "+err.Error())
					continue
				}
				if !slices.Equal(current, symbols) {
					changes <- errors.New("Binance symbol list changed")
					cancel()
					return
				}
			}
		}
	}()
	wg.Wait()
	select {
	case err := <-changes:
		return err
	default:
	}
	return ctx.Err()
}

func (f *Feed) syncClock(ctx context.Context) error {
	var result struct {
		ServerTime int64 `json:"serverTime"`
	}
	start := time.Now()
	if err := f.get(ctx, "/fapi/v1/time", &result); err != nil {
		return err
	}
	end := time.Now()
	if result.ServerTime == 0 {
		return errors.New("invalid Binance clock")
	}
	localMidpoint := start.Add(end.Sub(start) / 2)
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

func (f *Feed) get(ctx context.Context, path string, out any) error {
	requestCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, restBase+path, nil)
	if err != nil {
		return err
	}
	response, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 256))
		return fmt.Errorf("binance %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(out)
}

func (f *Feed) bookLoop(ctx context.Context, shard int) {
	backoff := time.Second
	for ctx.Err() == nil {
		f.cb.Gap(f.shardMembers[shard])
		connectedAt := time.Now()
		err := f.readBook(ctx, shard)
		if ctx.Err() != nil {
			return
		}
		f.status("reconnecting", fmt.Sprintf("book stream: %v", err))
		if time.Since(connectedAt) > time.Minute {
			backoff = time.Second
		}
		if !pause(ctx, backoff) {
			return
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

type bookTick struct {
	Type   string `json:"e"`
	Symbol string `json:"s"`
	Bid    string `json:"b"`
	BidQty string `json:"B"`
	Ask    string `json:"a"`
	AskQty string `json:"A"`
	Event  int64  `json:"E"`
	Trade  int64  `json:"T"`
}

func (f *Feed) readBook(ctx context.Context, shard int) error {
	connection, _, err := websocket.Dial(ctx, wsPublic, &websocket.DialOptions{HTTPClient: f.client})
	if err != nil {
		return err
	}
	defer connection.CloseNow()
	connection.SetReadLimit(2 << 20)
	subscribed := f.desiredStreams(shard)
	if err := writeSubscription(ctx, connection, "SUBSCRIBE", subscribed, 1); err != nil {
		return err
	}
	f.connection(1)
	defer f.connection(-1)
	writerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go f.manageSubscriptions(writerCtx, connection, shard, subscribed)
	go pingUntilDone(ctx, connection)
	for {
		readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		_, raw, err := connection.Read(readCtx)
		cancel()
		if err != nil {
			return err
		}
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope.Data) == 0 {
			continue
		}
		var tick bookTick
		if err := json.Unmarshal(envelope.Data, &tick); err != nil || tick.Symbol == "" {
			continue
		}
		bid, bidErr := strconv.ParseFloat(tick.Bid, 64)
		ask, askErr := strconv.ParseFloat(tick.Ask, 64)
		if bidErr != nil || askErr != nil || !f.volumeOK.Load() {
			continue
		}
		received := time.Now()
		event := tick.Event
		if event == 0 {
			event = tick.Trade
		}
		if event == 0 {
			event = received.UnixMilli()
		}
		eventTime := time.UnixMilli(event).Add(-time.Duration(f.clockOffset.Load()))
		if received.Sub(eventTime) > 10*time.Second || eventTime.After(received.Add(2*time.Second)) {
			return fmt.Errorf("stale book stream: %s event age %s", tick.Symbol, received.Sub(eventTime))
		}
		f.cb.Quote(core.Quote{Symbol: tick.Symbol, Bid: bid, Ask: ask, EventTime: eventTime, Received: received})
	}
}

func (f *Feed) volumeLoop(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		connectedAt := time.Now()
		err := f.readVolume(ctx)
		f.volumeOK.Store(false)
		f.cb.Gap(nil)
		if ctx.Err() != nil {
			return
		}
		f.status("reconnecting", fmt.Sprintf("volume stream: %v", err))
		if time.Since(connectedAt) > time.Minute {
			backoff = time.Second
		}
		if !pause(ctx, backoff) {
			return
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

func (f *Feed) readVolume(ctx context.Context) error {
	connection, _, err := websocket.Dial(ctx, wsMarket+"?streams=!miniTicker@arr", &websocket.DialOptions{HTTPClient: f.client})
	if err != nil {
		return err
	}
	defer connection.CloseNow()
	connection.SetReadLimit(8 << 20)
	f.volumeOK.Store(true)
	f.volumeOnce.Do(func() { close(f.volumeReady) })
	f.status("warming", "waiting for rule windows")
	go pingUntilDone(ctx, connection)
	for {
		readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		_, raw, err := connection.Read(readCtx)
		cancel()
		if err != nil {
			return err
		}
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope.Data) == 0 {
			continue
		}
		var ticks []struct {
			Type   string `json:"e"`
			Symbol string `json:"s"`
			Quote  string `json:"q"`
			Event  int64  `json:"E"`
		}
		if err := json.Unmarshal(envelope.Data, &ticks); err != nil {
			continue
		}
		if len(ticks) > 0 {
			eventTime := time.UnixMilli(ticks[0].Event).Add(-time.Duration(f.clockOffset.Load()))
			if ticks[0].Event > 0 && time.Since(eventTime) > 10*time.Second {
				return fmt.Errorf("stale volume stream: %s", time.Since(eventTime))
			}
		}
		values := make(map[string]float64, len(ticks))
		for _, tick := range ticks {
			if volume, err := strconv.ParseFloat(tick.Quote, 64); err == nil {
				values[tick.Symbol] = volume
			}
		}
		f.cb.Volume(values)
		f.updateEligibility(values)
	}
}

func (f *Feed) updateEligibility(values map[string]float64) {
	changed := make(map[int]bool)
	f.mu.Lock()
	for symbol, volume := range values {
		if !f.known[symbol] {
			continue
		}
		f.volumes[symbol] = volume
		wanted := volume >= f.minVolume
		if f.eligible[symbol] == wanted {
			continue
		}
		f.eligible[symbol] = wanted
		index := sort.SearchStrings(f.symbols, symbol)
		changed[index/shardSize] = true
	}
	f.mu.Unlock()
	for shard := range changed {
		select {
		case f.updates[shard] <- struct{}{}:
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

func (f *Feed) manageSubscriptions(ctx context.Context, connection *websocket.Conn, shard int, initial []string) {
	current := make(map[string]bool, len(initial))
	for _, stream := range initial {
		current[stream] = true
	}
	requestID := 2
	for {
		select {
		case <-ctx.Done():
			return
		case <-f.updates[shard]:
			desired := f.desiredStreams(shard)
			next := make(map[string]bool, len(desired))
			added := make([]string, 0)
			removed := make([]string, 0)
			for _, stream := range desired {
				next[stream] = true
				if !current[stream] {
					added = append(added, stream)
				}
			}
			for stream := range current {
				if !next[stream] {
					removed = append(removed, stream)
				}
			}
			if len(removed) > 0 {
				if err := writeSubscription(ctx, connection, "UNSUBSCRIBE", removed, requestID); err != nil {
					connection.CloseNow()
					return
				}
				requestID++
			}
			if len(added) > 0 {
				symbols := make([]string, len(added))
				for index, stream := range added {
					symbols[index] = strings.ToUpper(strings.TrimSuffix(stream, "@bookTicker"))
				}
				f.cb.Gap(symbols)
				if err := writeSubscription(ctx, connection, "SUBSCRIBE", added, requestID); err != nil {
					connection.CloseNow()
					return
				}
				requestID++
			}
			current = next
		}
	}
}

func (f *Feed) connection(delta int) {
	f.mu.Lock()
	f.active += delta
	f.mu.Unlock()
	f.status("warming", "waiting for rule windows")
}

func (f *Feed) status(state, detail string) {
	f.mu.Lock()
	health := core.Health{
		State: state, Detail: detail, Connections: f.active,
		Symbols: len(f.symbols), UpdatedAtMs: time.Now().UnixMilli(),
	}
	if state == "warming" && f.active < f.shards {
		health.State = "reconnecting"
	}
	f.mu.Unlock()
	f.cb.Health(health)
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

func pingUntilDone(ctx context.Context, connection *websocket.Conn) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := connection.Ping(pingCtx)
			cancel()
			if err != nil {
				connection.CloseNow()
				return
			}
		}
	}
}

func pause(ctx context.Context, duration time.Duration) bool {
	delay := duration + time.Duration(rand.Int63n(int64(duration/2)+1))
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
