package desktop

import (
	"slices"
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
		c.pingSamples = nil
	}
	health.Storage = c.pipelineHealth()
	health.Timings = c.recentTimings()
	health.TerminalErrors = c.terminalErrors.Load()
	health.TerminalWaitMs = c.terminalWaitMs.Load()
	health.TerminalRequestMs = c.terminalRequestMs.Load()
	health.RenderDelayMs = c.renderDelayMs.Load()
	for _, queue := range c.terminalQueues {
		health.TerminalQueued += len(queue)
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
	c.pingSamples = append(c.pingSamples, elapsed.Milliseconds())
	if len(c.pingSamples) > 5 {
		c.pingSamples = c.pingSamples[1:]
	}
	sorted := slices.Clone(c.pingSamples)
	slices.Sort(sorted)
	middle := len(sorted) / 2
	ms := sorted[middle]
	if len(sorted)%2 == 0 {
		ms = sorted[middle-1] + (ms-sorted[middle-1])/2
	}
	c.health.PingMs = &ms
	c.health.UpdatedAtMs = time.Now().UnixMilli()
	health := c.health
	c.mu.Unlock()
	c.emit("health", health)
}
