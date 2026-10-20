//go:build integration

package binance

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"cryptotape/internal/core"
)

func TestPublicFuturesFeed(t *testing.T) {
	var quotes atomic.Int64
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	feed, err := NewFeed("", 80_000_000, Callbacks{
		Quote:  func(core.Quote) { quotes.Add(1); cancel() },
		Volume: func(map[string]float64) {},
		Gap:    func([]string) {},
		Health: func(core.Health) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := feed.Run(ctx); err != nil && err != context.DeadlineExceeded && err != context.Canceled {
		t.Fatal(err)
	}
	if quotes.Load() == 0 {
		t.Fatal("no bookTicker events received")
	}
}
