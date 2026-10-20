package desktop

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"cryptotape/internal/core"
)

const candidateCapacity = 8192

type queuedCandidate struct {
	candidate core.Candidate
	id        uint64
	queued    time.Time
}

func (c *Controller) startPipeline() {
	c.persistenceCtx, c.persistenceCancel = context.WithCancel(context.Background())
	c.candidates = make(chan queuedCandidate, candidateCapacity)
	c.notifications = make(chan core.Alert, candidateCapacity)
	c.pending = make(map[uint64]time.Time)
	c.traces = make(map[int64]*alertTrace)
	c.startTerminals()
	c.pipelineWG.Add(2)
	go func() {
		defer c.pipelineWG.Done()
		defer close(c.notifications)
		for first := range c.candidates {
			batch := []queuedCandidate{first}
			for len(batch) < 64 {
				select {
				case next, ok := <-c.candidates:
					if ok {
						batch = append(batch, next)
					} else {
						goto save
					}
				default:
					goto save
				}
			}
		save:
			c.saveCandidates(batch)
		}
	}()
	go func() {
		defer c.pipelineWG.Done()
		for alert := range c.notifications {
			c.emit("alert:new", alert)
			c.actionWG.Done()
		}
	}()
}

func (c *Controller) saveCandidates(batch []queuedCandidate) {
	candidates := make([]core.Candidate, len(batch))
	for i, job := range batch {
		candidates[i] = job.candidate
	}
	started := time.Now()
	alerts, err := c.store.InsertAlerts(c.persistenceCtx, candidates)
	committed := time.Now()
	c.queueMu.Lock()
	c.queueHealth.WriteMs = time.Since(started).Milliseconds()
	for _, job := range batch {
		delete(c.pending, job.id)
	}
	if err != nil {
		c.queueHealth.Unsaved += uint64(len(batch))
		c.queueHealth.Error = "alert storage: " + err.Error()
		if c.pipelineError == nil {
			c.pipelineError = err
		}
	}
	c.queueMu.Unlock()
	if err != nil {
		slog.Error("save alerts", "count", len(batch), "error", err)
		for range batch {
			c.actionWG.Done()
		}
		return
	}
	for i, alert := range alerts {
		c.recordCommit(alert, batch[i].candidate, committed)

		c.enqueueAutomatic(alert, batch[i].candidate.Expires)
		if err := c.audio.PlayAlert(alert.ID, alert.Sound, alert.Volume); err != nil {
			slog.Error("play alert", "error", err)
		}
		// slow native window operations cannot hold the persistence worker.
		select {
		case c.notifications <- alert:
		default:
			c.queueMu.Lock()
			c.queueHealth.Error = "notification queue overflow; alerts retained in history"
			c.pipelineError = errors.New(c.queueHealth.Error)
			c.queueMu.Unlock()
			c.actionWG.Done()
		}
	}
}

func (c *Controller) pipelineHealth() core.QueueHealth {
	c.queueMu.Lock()
	defer c.queueMu.Unlock()
	result := c.queueHealth
	result.Size = len(c.pending)
	for _, at := range c.pending {
		result.OldestMs = max(result.OldestMs, time.Since(at).Milliseconds())
	}
	return result
}

func (c *Controller) MonitorID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.settings.MonitorID
}
