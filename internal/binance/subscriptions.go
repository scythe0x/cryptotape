package binance

import (
	"context"
	"errors"
	"fmt"
	"github.com/coder/websocket"
	"sync"
	"time"
)

type subscriptions struct {
	connection *websocket.Conn
	mu         sync.Mutex
	pending    map[int64]*subscriptionResult
	sequence   int64
	lastWrite  time.Time
}

func newSubscriptions(c *websocket.Conn) *subscriptions {
	return &subscriptions{connection: c, pending: make(map[int64]*subscriptionResult)}
}

type subscriptionResult struct {
	done    chan error
	applied chan struct{}
}

func (s *subscriptions) ack(ctx context.Context, id int64, frame bookFrame) {
	s.mu.Lock()
	pending := s.pending[id]
	s.mu.Unlock()
	if pending == nil {
		return
	}
	var err error
	if frame.Code != nil {
		err = fmt.Errorf("subscription %d: %d %s", id, *frame.Code, frame.Msg)
	} else if string(frame.Result) != "null" {
		err = errors.New("invalid subscription acknowledgement")
	}
	pending.done <- err
	select {
	case <-pending.applied:
	case <-ctx.Done():
	}
}

func (s *subscriptions) send(ctx context.Context, method string, streams []string, apply func() bool) error {
	if len(streams) == 0 {
		return nil
	}
	if len(streams) > 1024 {
		return errors.New("too many websocket streams")
	}
	if delay := time.Until(s.lastWrite.Add(200 * time.Millisecond)); delay > 0 && !pause(ctx, delay) {
		return context.Cause(ctx)
	}
	s.sequence++
	id := s.sequence
	pending := &subscriptionResult{done: make(chan error, 1), applied: make(chan struct{})}
	defer close(pending.applied)
	s.mu.Lock()
	s.pending[id] = pending
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.pending, id); s.mu.Unlock() }()
	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	s.lastWrite = time.Now()
	if err := writeSubscription(requestCtx, s.connection, method, streams, int(id)); err != nil {
		return err
	}
	select {
	case err := <-pending.done:
		if err == nil && apply != nil && !apply() {
			return context.Canceled
		}
		return err
	case <-requestCtx.Done():
		return fmt.Errorf("subscription acknowledgement: %w", requestCtx.Err())
	}
}

func (f *Feed) reconcileSubscriptions(ctx context.Context, shard int, control *subscriptions, confirmed func(int) bool) error {
	f.mu.Lock()
	updates := f.updates[shard]
	f.mu.Unlock()
	current := make(map[string]bool)
	for {
		if ctx.Err() != nil {
			return context.Cause(ctx)
		}
		desired := f.desiredStreams(shard)
		if len(desired) > 1024 {
			return errors.New("too many websocket streams")
		}
		next := make(map[string]bool, len(desired))
		var added, removed []string
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
		if err := control.send(ctx, "UNSUBSCRIBE", removed, nil); err != nil {
			return err
		}
		if err := control.send(ctx, "SUBSCRIBE", added, func() bool { return confirmed(len(next)) }); err != nil {
			return err
		}

		current = next
		if !confirmed(len(current)) {
			return context.Cause(ctx)
		}
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case <-updates:
		}
	}
}
