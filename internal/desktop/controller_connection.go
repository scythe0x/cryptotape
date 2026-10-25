package desktop

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"cryptotape/internal/binance"
	"cryptotape/internal/core"
)

func (c *Controller) Reconnect() {
	c.engine.ResetAll()
	c.restartFeed()
}

func (c *Controller) restartFeed() {
	c.mu.Lock()
	if c.root == nil {
		c.mu.Unlock()
		return
	}
	if c.cancel != nil {
		c.cancel()
	}
	ctx, cancel := context.WithCancel(c.root)
	c.cancel = cancel
	proxy, err := c.settings.ProxyURL()
	if err != nil {
		c.mu.Unlock()
		c.setHealth(core.Health{State: "error", Detail: "invalid proxy profile", ViaProxy: true, UpdatedAtMs: time.Now().UnixMilli()})
		return
	}
	c.feed = nil
	minVolume := core.MinimumVolume(c.rules)
	version := c.version.Add(1)
	c.feedWG.Add(1)
	c.mu.Unlock()
	c.setFeedHealth(version, core.Health{State: "connecting", Detail: "opening market streams", ViaProxy: proxy != "", UpdatedAtMs: time.Now().UnixMilli()})
	go func() {
		defer c.feedWG.Done()
		c.feedLoop(ctx, proxy, minVolume, version)
	}()
}

func (c *Controller) feedLoop(ctx context.Context, proxy string, minVolume float64, version uint64) {
	err := binance.RetryConnection(ctx, func() (bool, error) {
		var hadQuotes atomic.Bool
		feed, err := binance.NewFeed(proxy, minVolume, binance.Callbacks{
			Symbols: func(symbols []string) {
				if c.version.Load() == version {
					c.updateListingCatalog(symbols)
				}
			},
			Quote: func(quote core.Quote) {
				hadQuotes.Store(true)
				if c.version.Load() == version {
					c.onQuote(quote)
				}
			},
			Volume: func(values map[string]float64) {
				if c.version.Load() == version {
					c.engine.SetVolumes(values)
				}
			},
			Gap: func(symbols []string) {
				if c.version.Load() != version {
					return
				}
				if symbols == nil {
					c.engine.ResetAll()
				} else {
					c.engine.Reset(symbols)
				}
			},
			Ping: func(elapsed time.Duration) { c.reportPing(version, elapsed) },
			Health: func(health core.Health) {
				if c.version.Load() == version {
					health.ViaProxy = proxy != ""
					if proxy != "" && (health.State == "error" || health.State == "reconnecting" || health.State == "degraded") {
						health.Detail = "proxy connection failed"
					}
					c.setFeedHealth(version, health)
				}
			},
		})
		if err == nil {
			c.mu.Lock()
			if c.version.Load() == version {
				c.feed = feed
				feed.SetMinimumVolume(core.MinimumVolume(c.rules))
			}
			c.mu.Unlock()
			err = feed.Run(ctx)
		}
		return hadQuotes.Load(), err
	}, func(err error, retry bool) {
		if c.version.Load() != version {
			return
		}
		state := "error"
		if retry {
			state = "reconnecting"
		}
		detail := "binance connection failed"
		if proxy != "" {
			detail = "proxy connection failed"
		}
		c.setFeedHealth(version, core.Health{State: state, Detail: detail, ViaProxy: proxy != "", UpdatedAtMs: time.Now().UnixMilli()})
	})
	if err != nil && ctx.Err() == nil {
		c.mu.Lock()
		if c.version.Load() == version {
			c.feed = nil
		}
		c.mu.Unlock()
	}

}

// reuse the market route and REST admission, including the selected proxy.
func (c *Controller) ClockOffset(ctx context.Context) (float64, error) {
	c.mu.RLock()
	feed := c.feed
	c.mu.RUnlock()
	if feed == nil {
		return 0, errors.New("market connection unavailable")
	}
	return feed.ClockOffset(ctx)
}
