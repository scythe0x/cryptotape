package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"cryptotape/internal/core"
	"github.com/coder/websocket"
)

type streamState struct {
	health    core.StreamHealth
	owner     uint64
	received  time.Time
	age       time.Duration
	histogram ageHistogram
}

type bookTick struct {
	Type     string `json:"e"`
	Symbol   string `json:"s"`
	Bid      string `json:"b"`
	BidQty   string `json:"B"`
	Ask      string `json:"a"`
	AskQty   string `json:"A"`
	Event    int64  `json:"E"`
	Trade    int64  `json:"T"`
	UpdateID uint64 `json:"u"`
}

type bookFrame struct {
	Data   bookTick        `json:"data"`
	ID     *int64          `json:"id"`
	Result json.RawMessage `json:"result"`
	Code   *int            `json:"code"`
	Msg    string          `json:"msg"`
}

func (f *Feed) bookLoop(ctx context.Context, shard int) error {
	return RetryConnection(ctx, func() (bool, error) {
		f.mu.Lock()
		members := append([]string(nil), f.shardMembers[shard]...)
		f.mu.Unlock()
		f.cb.Gap(members)
		return f.readRotating(ctx, shard)
	}, func(err error, retry bool) {
		state := "error"
		if retry {
			state = "reconnecting"
		}
		f.setStream(shard, func(s *streamState) { s.health.State, s.health.Detail = state, err.Error(); s.health.Subscriptions = 0 })
		f.status("warming", "signals live")
	})
}

func (f *Feed) setStream(shard int, change func(*streamState)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.streams == nil {
		f.streams = make(map[int]*streamState)
	}
	state := f.streams[shard]
	if state == nil {
		state = &streamState{health: core.StreamHealth{Shard: shard, State: "connecting"}}
		f.streams[shard] = state
	}
	change(state)
}

func (f *Feed) readSession(ctx context.Context, shard int, token uint64, ready func() bool) (bool, error) {
	ctx, cancel := context.WithCancel(ctx)
	connection, _, err := websocket.Dial(ctx, wsPublic, &websocket.DialOptions{HTTPClient: f.client})
	if err != nil {
		cancel()
		return false, err
	}
	defer connection.CloseNow()
	connection.SetReadLimit(2 << 20)
	control := newSubscriptions(connection)
	var workers sync.WaitGroup
	defer workers.Wait()
	// cancellation must run before waiting for the session workers.
	defer cancel()
	failed := make(chan error, 1)
	var confirmed atomic.Bool
	workers.Add(2)
	go func() {
		defer workers.Done()
		err := f.reconcileSubscriptions(ctx, shard, control, func(count int) bool {
			if !confirmed.Load() {
				if !ready() {
					return false
				}
				confirmed.Store(true)
			}
			f.setStream(shard, func(s *streamState) {
				if s.owner == token {
					s.health.State = "connected"
					if count == 0 {
						s.health.State = "idle"
					}
					s.health.Subscriptions = count
				}
			})
			f.status("warming", "signals live")
			return true
		})
		if err != nil {
			select {
			case failed <- err:
			default:
			}
			connection.CloseNow()
		}
	}()
	go func() { defer workers.Done(); pingUntilDone(ctx, connection, f.cb.Ping) }()
	healthy := false
	for {
		_, raw, err := connection.Read(ctx)
		received := time.Now()
		if err != nil {
			select {
			case failure := <-failed:
				err = failure
			default:
			}
			return healthy, err
		}
		var frame bookFrame
		if err := json.Unmarshal(raw, &frame); err != nil {
			continue
		}
		if frame.ID != nil {
			control.ack(ctx, *frame.ID, frame)
			continue
		}
		tick := frame.Data
		if !confirmed.Load() || tick.Symbol == "" {
			continue
		}
		f.mu.Lock()
		eligible := f.eligible[tick.Symbol]
		state := f.streams[shard]
		selected := state != nil && state.owner == token
		f.mu.Unlock()
		if !eligible || !selected {
			continue
		}
		event := tick.Event
		if event == 0 {
			event = tick.Trade
		}
		if event == 0 {
			continue
		}
		bid, bidErr := strconv.ParseFloat(tick.Bid, 64)
		ask, askErr := strconv.ParseFloat(tick.Ask, 64)
		if bidErr != nil || askErr != nil || bid <= 0 || ask < bid || math.IsNaN(bid) || math.IsNaN(ask) || math.IsInf(bid, 0) || math.IsInf(ask, 0) {
			continue
		}
		eventTime := time.UnixMilli(event)
		offset := time.Duration(f.clockOffset.Load())
		age := received.Add(offset).Sub(eventTime)
		f.setStream(shard, func(s *streamState) {
			if s.owner == token {
				s.received, s.age = received, age
				s.histogram.add(received, age)
			}
		})
		if age > 10*time.Second || age < -2*time.Second {
			return healthy, fmt.Errorf("stale book stream: %s event age %s", tick.Symbol, age)
		}
		healthy = true
		f.cb.Quote(core.Quote{Symbol: tick.Symbol, Bid: bid, Ask: ask, UpdateID: tick.UpdateID, EventTime: eventTime, Received: received, ClockOffset: offset})
	}
}
