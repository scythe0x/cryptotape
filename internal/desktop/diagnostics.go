package desktop

import (
	"sort"
	"time"

	"cryptotape/internal/core"
)

const traceCapacity = 128

type alertTrace struct {
	timing   core.AlertTiming
	received time.Time
}

func (c *Controller) recordCommit(alert core.Alert, candidate core.Candidate, committed time.Time) {
	c.traceMu.Lock()
	defer c.traceMu.Unlock()
	if len(c.traceOrder) == traceCapacity {
		delete(c.traces, c.traceOrder[c.traceCursor])
		c.traceOrder[c.traceCursor] = alert.ID
		c.traceCursor = (c.traceCursor + 1) % traceCapacity
	} else {
		c.traceOrder = append(c.traceOrder, alert.ID)
	}
	c.traces[alert.ID] = &alertTrace{received: candidate.Received, timing: core.AlertTiming{
		ID: alert.ID, ReceivedAtMs: alert.ReceivedAtMs, DetectedAtMs: alert.DetectedAtMs, CommittedAtMs: committed.UnixMilli(),
	}}
}

func (c *Controller) markTerminal(id int64, completed bool) {
	c.traceMu.Lock()
	defer c.traceMu.Unlock()
	if trace := c.traces[id]; trace != nil {
		if completed {
			trace.timing.TerminalDoneAtMs = time.Now().UnixMilli()
		} else if trace.timing.TerminalSentAtMs == 0 {
			trace.timing.TerminalSentAtMs = time.Now().UnixMilli()
		}
	}
}

func (c *Controller) AlertRendered(id int64, view string) {
	c.traceMu.Lock()
	defer c.traceMu.Unlock()
	trace := c.traces[id]
	if trace == nil {
		return
	}
	switch view {
	case "feed":
		if trace.timing.FeedRenderedAtMs == 0 {
			trace.timing.FeedRenderedAtMs = time.Now().UnixMilli()
		}
	case "overlay":
		if trace.timing.OverlayRenderedAtMs == 0 {
			trace.timing.OverlayRenderedAtMs = time.Now().UnixMilli()
			c.renderDelayMs.Store(time.Since(trace.received).Milliseconds())
		}
	}
}

func (c *Controller) recentTimings() []core.AlertTiming {
	c.traceMu.Lock()
	defer c.traceMu.Unlock()
	result := make([]core.AlertTiming, 0, len(c.traces))
	for _, trace := range c.traces {
		result = append(result, trace.timing)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID > result[j].ID })
	return result
}
