package desktop

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"cryptotape/internal/core"
	"cryptotape/internal/listings"
)

type ListingSource struct {
	Endpoint      string             `json:"endpoint"`
	KeyConfigured bool               `json:"keyConfigured"`
	Health        core.ListingHealth `json:"health"`
}

func (c *Controller) ListingSource() ListingSource {
	c.listingMu.Lock()
	defer c.listingMu.Unlock()
	return ListingSource{Endpoint: c.listingConfig.Endpoint, KeyConfigured: c.listingConfig.Key != "", Health: c.listingHealth}
}

func (c *Controller) SaveListingSource(endpoint, key string) error {
	c.listingRestartMu.Lock()
	c.listingMu.Lock()
	config := c.listingConfig
	c.listingMu.Unlock()
	config.Endpoint = endpoint
	if key != "" {
		config.Key = strings.TrimSpace(key)
	}
	err := listings.SaveConfig(c.listingConfigPath, config)
	if err == nil {
		c.listingMu.Lock()
		c.listingConfig = config
		c.listingConfigError = false
		c.listingMu.Unlock()
	}
	c.listingRestartMu.Unlock()
	if err == nil {
		c.restartListings()
	}
	return err
}

func (c *Controller) ClearListingKey() error {
	c.listingRestartMu.Lock()
	c.listingMu.Lock()
	config := c.listingConfig
	c.listingMu.Unlock()
	config.Key = ""
	err := listings.SaveConfig(c.listingConfigPath, config)
	if err == nil {
		c.listingMu.Lock()
		c.listingConfig = config
		c.listingConfigError = false
		c.listingMu.Unlock()
	}
	c.listingRestartMu.Unlock()
	if err == nil {
		c.restartListings()
	}
	return err
}

func (c *Controller) updateListingCatalog(symbols []string) {
	catalog := listings.NewCatalog(symbols)
	c.listingMu.Lock()
	c.listingCatalog = catalog
	c.listingMu.Unlock()
}

func (c *Controller) stopListings() {
	c.listingRestartMu.Lock()
	defer c.listingRestartMu.Unlock()
	c.stopListingsLocked()
}

func (c *Controller) stopListingsLocked() {
	c.listingMu.Lock()
	cancel, done := c.listingCancel, c.listingDone
	c.listingCancel, c.listingDone = nil, nil
	c.listingMu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}

func (c *Controller) restartListings() {
	c.listingRestartMu.Lock()
	defer c.listingRestartMu.Unlock()
	c.stopListingsLocked()
	c.mu.RLock()
	root := c.root
	enabled := slices.ContainsFunc(c.rules, func(rule core.Rule) bool { return rule.IsListing() })
	c.mu.RUnlock()
	c.listingMu.Lock()
	config := c.listingConfig
	invalid := c.listingConfigError
	c.listingHealth.Plan, c.listingHealth.DelayMs = "", 0
	c.listingMu.Unlock()
	if invalid {
		c.listingStatus("error", "INVALID_CONFIG")
		return
	}
	if config.Key == "" {
		c.listingStatus("disabled", "KEY_REQUIRED")
		return
	}
	if !enabled {
		c.listingStatus("disabled", "NO_RULES")
		return
	}
	if root == nil || root.Err() != nil {
		c.listingStatus("disabled", "NOT_STARTED")
		return
	}
	ctx, cancel := context.WithCancel(root)
	done := make(chan struct{})
	c.listingMu.Lock()
	c.listingCancel, c.listingDone = cancel, done
	c.listingMu.Unlock()
	c.listingStatus("connecting", "CONNECTING")
	go func() {
		defer close(done)
		client, err := listings.NewClient(config, listings.Callbacks{
			Ready: func(subscription listings.Subscription) {
				if ctx.Err() != nil {
					return
				}
				c.listingMu.Lock()
				c.listingHealth.Plan, c.listingHealth.DelayMs = subscription.Plan, *subscription.DelayMs
				c.listingMu.Unlock()
			},
			Status: func(state, code string) {
				if ctx.Err() == nil {
					c.listingStatus(state, code)
				}
			},
			Event: func(event listings.Event, at time.Time) {
				if ctx.Err() == nil {
					c.onListing(event, at)
				}
			},
		})
		if err != nil {
			c.listingStatus("error", "INVALID_CONFIG")
			return
		}
		// the client reports sanitized errors; listing failures never cancel Binance.
		if err := client.Run(ctx); err != nil && ctx.Err() == nil {
			code := "FEED_STOPPED"
			var failure *listings.FeedError
			if errors.As(err, &failure) {
				code = failure.Code
			}
			c.listingStatus("error", code)
		}
	}()
}

func (c *Controller) listingStatus(state, code string) {
	c.listingMu.Lock()
	c.listingHealth.State, c.listingHealth.Code = state, code
	health := c.listingHealth
	c.listingMu.Unlock()
	c.emit("listing:health", health)
}

func (c *Controller) onListing(event listings.Event, received time.Time) {
	assets, err := event.Listings(received)
	c.listingMu.Lock()
	c.listingHealth.Received++
	c.listingHealth.LastEventAtMs = received.UnixMilli()
	if err != nil {
		c.listingHealth.Rejected++
	}
	if event.Sent >= event.Detected && event.Detected > 0 {
		c.listingHealth.ProviderWaitMs = (event.Sent - event.Detected) / 1000
		c.listingHealth.DeliveryMs = received.Sub(time.UnixMicro(event.Sent)).Milliseconds()
	}
	catalog := c.listingCatalog
	c.listingMu.Unlock()
	if err == nil {
		c.admitListings(assets, catalog, received)
	}
	c.listingMu.Lock()
	health := c.listingHealth
	c.listingMu.Unlock()
	c.emit("listing:health", health)
}

func (c *Controller) admitListings(assets []core.ListingDetails, catalog listings.Catalog, received time.Time) {
	c.pipelineMu.RLock()
	defer c.pipelineMu.RUnlock()
	if c.closing {
		return
	}
	c.mu.RLock()
	rules := append([]core.Rule(nil), c.rules...)
	c.mu.RUnlock()
	for _, asset := range assets {
		symbol, ok := catalog.Resolve(asset.Asset)
		if !ok && len(catalog[asset.Asset]) == 0 {
			symbol, ok = asset.Asset+"USDT", true
		}
		if !ok {
			c.listingMu.Lock()
			c.listingHealth.Unmatched++
			c.listingMu.Unlock()
			continue
		}
		for _, rule := range rules {
			if !rule.IsListing() || !slices.Contains(rule.ListingTypes, asset.MarketType) || (len(rule.ListingExchanges) > 0 && !slices.Contains(rule.ListingExchanges, asset.Exchange)) {
				continue
			}
			detected := time.UnixMicro(asset.DetectedTimeUs)
			detail := asset
			c.enqueueCandidate(core.Candidate{Rule: rule, Symbol: symbol, Listing: &detail, EventTime: detected, Received: received, Detected: time.Now(), Expires: received.Add(10*time.Second - received.Sub(detected))})
		}
	}
}
