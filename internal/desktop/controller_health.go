package desktop

import (
	"time"

	"cryptotape/internal/core"
)

func (c *Controller) setHealth(health core.Health) { c.setFeedHealth(0, health) }

// serialize health events so an old stream cannot overwrite a newer session's failure.
func (c *Controller) setFeedHealth(version uint64, health core.Health) {
	c.healthMu.Lock()
	defer c.healthMu.Unlock()
	if health.State == "warming" {
		health.Ready = c.engine.ReadyCount(time.Now())
		health.State = "connected"
		health.Detail = "signals live"
	}
	c.mu.Lock()
	if version != 0 && c.version.Load() != version {
		c.mu.Unlock()
		return
	}
	if health.State == "connected" {
		health.PingMs = c.health.PingMs
	} else {
		health.PingMs = nil
	}
	c.health = health
	c.mu.Unlock()
	c.emit("health", health)
}

func (c *Controller) reportPing(version uint64, elapsed time.Duration) {
	c.healthMu.Lock()
	defer c.healthMu.Unlock()
	c.mu.Lock()
	if c.version.Load() != version || c.health.State != "connected" {
		c.mu.Unlock()
		return
	}
	ms := elapsed.Milliseconds()
	c.health.PingMs = &ms
	c.health.UpdatedAtMs = time.Now().UnixMilli()
	health := c.health
	c.mu.Unlock()
	c.emit("health", health)
}
