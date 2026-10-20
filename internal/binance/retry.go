package binance

import (
	"context"
	"errors"
	"time"
)

var ErrRetriesExhausted = errors.New("connection retries exhausted")

// connect reports whether fresh data arrived. only a stable, productive session resets the budget.
func RetryConnection(ctx context.Context, connect func() (bool, error), failed func(error, bool)) error {
	attempt := 0
	for {
		if ctx.Err() != nil {
			return context.Cause(ctx)
		}
		started := time.Now()
		healthy, err := connect()
		if ctx.Err() != nil {
			return context.Cause(ctx)
		}
		if err == nil {
			return nil
		}
		if healthy && time.Since(started) >= time.Minute {
			attempt = 0
		}
		retry := attempt < 4 && !errors.Is(err, ErrRetriesExhausted)
		failed(err, retry)
		if !retry {
			return errors.Join(ErrRetriesExhausted, err)
		}
		if !pause(ctx, time.Second<<attempt) {
			return context.Cause(ctx)
		}
		attempt++
	}
}
