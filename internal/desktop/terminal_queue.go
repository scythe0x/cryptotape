package desktop

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"cryptotape/internal/core"
)

type terminalJob struct {
	alertID  int64
	symbol   string
	terminal string
	queued   time.Time
	expires  time.Time
	complete func(error, string)
}

func (c *Controller) startTerminals() {
	c.startTerminalResults()
	c.terminalQueues = make(map[string]chan terminalJob)
	for terminal, workers := range map[string]int{"vataga": 32, "tiger": 1} {
		queue := make(chan terminalJob, candidateCapacity)
		c.terminalQueues[terminal] = queue
		for range workers {
			c.terminalWG.Add(1)
			go func() {
				defer c.terminalWG.Done()
				for job := range queue {
					c.terminalWaitMs.Store(time.Since(job.queued).Milliseconds())
					status := "ok"
					var err error
					if !time.Now().Before(job.expires) {
						status, err = "stale", errors.New("terminal command expired in queue")
					} else {
						deadline := time.Now().Add(1500 * time.Millisecond)
						if job.expires.Before(deadline) {
							deadline = job.expires
						}
						ctx, cancel := context.WithDeadline(context.Background(), deadline)
						started := time.Now()
						c.markTerminal(job.alertID, false)
						err = c.openInTerminal(ctx, job.symbol, job.terminal)
						c.terminalRequestMs.Store(time.Since(started).Milliseconds())
						c.markTerminal(job.alertID, true)
						cancel()
						if err != nil {
							status = "error"
						}
					}
					if err != nil {
						c.terminalErrors.Add(1)
					}
					job.complete(err, status)
					c.actionWG.Done()
				}
			}()
		}
	}
}

func (c *Controller) enqueueTerminal(job terminalJob) {
	c.actionWG.Add(1)
	select {
	case c.terminalQueues[job.terminal] <- job:
	default:
		c.terminalErrors.Add(1)
		job.complete(errors.New(job.terminal+" queue overflow"), "overflow")
		c.actionWG.Done()
	}
}

func (c *Controller) enqueueAutomatic(alert core.Alert, expires time.Time) {
	if len(alert.Terminals) == 0 {
		return
	}
	var mu sync.Mutex
	remaining := len(alert.Terminals)
	status := "ok"
	var failures []error
	for _, terminal := range alert.Terminals {
		c.enqueueTerminal(terminalJob{alertID: alert.ID, symbol: alert.Symbol, terminal: terminal, queued: time.Now(), expires: expires,
			complete: func(err error, outcome string) {
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					failures = append(failures, fmt.Errorf("%s: %w", terminal, err))
					status = outcome
				}
				remaining--
				if remaining != 0 {
					return
				}
				detail := ""
				if err := errors.Join(failures...); err != nil {
					detail = err.Error()
				}
				c.saveTerminalResult(alert.ID, status, detail)
			},
		})
	}
}
