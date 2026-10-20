package binance

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type sessionEvent struct {
	token   uint64
	ready   chan struct{}
	healthy bool
	err     error
}

func (f *Feed) readRotating(ctx context.Context, shard int) (bool, error) {
	duration := f.rotationAfter
	if duration == 0 {
		duration = 23*time.Hour + 50*time.Minute
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	events := make(chan sessionEvent, 8)
	var wg sync.WaitGroup
	sessions := make(map[uint64]context.CancelFunc)
	defer func() {
		for _, cancel := range sessions {
			cancel()
		}
		wg.Wait()
	}()
	var sequence, owner, replacement uint64
	launch := func() uint64 {
		sequence++
		token := sequence
		child, cancel := context.WithCancel(ctx)
		sessions[token] = cancel
		wg.Add(1)
		go func() {
			defer wg.Done()
			healthy, err := f.readSession(child, shard, token, func() bool {
				ready := make(chan struct{})
				select {
				case events <- sessionEvent{token: token, ready: ready}:
				case <-child.Done():
					return false
				}
				select {
				case <-ready:
					return true
				case <-child.Done():
					return false
				}
			})
			select {
			case events <- sessionEvent{token: token, healthy: healthy, err: err}:
			case <-ctx.Done():
			}
		}()
		return token
	}
	replacement = launch()
	for {
		select {
		case <-ctx.Done():
			return owner != 0, context.Cause(ctx)
		case <-timer.C:
			if replacement == 0 {
				replacement = launch()
			}
		case event := <-events:
			if event.ready != nil {
				previous := owner
				owner, replacement = event.token, 0
				f.setStream(shard, func(s *streamState) { s.owner = owner; s.health.Detail = "" })
				close(event.ready)
				if previous != 0 {
					sessions[previous]()
					delete(sessions, previous)
				}
				timer.Reset(duration)
				// a reconciliation notification may have been consumed by the old reader.
				f.mu.Lock()
				updates := f.updates[shard]
				f.mu.Unlock()
				select {
				case updates <- struct{}{}:
				default:
				}
				continue
			}
			cancel, exists := sessions[event.token]
			if !exists {
				continue
			}
			cancel()
			delete(sessions, event.token)
			if event.token == owner || owner == 0 {
				return event.healthy, event.err
			}
			// a failed replacement leaves the confirmed old connection in service.
			replacement = 0
			f.setStream(shard, func(s *streamState) { s.health.Detail = "rotation: " + fmt.Sprint(event.err) })
			timer.Reset(time.Minute)
		}
	}
}
