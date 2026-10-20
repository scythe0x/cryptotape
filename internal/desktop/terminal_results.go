package desktop

import (
	"errors"
	"fmt"

	"cryptotape/internal/core"
)

func (c *Controller) startTerminalResults() {
	c.terminalResults = make(chan core.AlertUpdate, candidateCapacity)
	c.terminalWG.Add(1)
	go func() {
		defer c.terminalWG.Done()
		for first := range c.terminalResults {
			batch := []core.AlertUpdate{first}
			for len(batch) < 64 {
				select {
				case next, ok := <-c.terminalResults:
					if !ok {
						goto save
					}
					batch = append(batch, next)
				default:
					goto save
				}
			}
		save:
			if err := c.store.UpdateTerminalsContext(c.persistenceCtx, batch); err != nil {
				c.terminalResultError(fmt.Errorf("save terminal status: %w", err), len(batch))
			} else {
				for _, update := range batch {
					c.emit("alert:update", update)
				}
			}
			for range batch {
				c.actionWG.Done()
			}
		}
	}()
}

func (c *Controller) saveTerminalResult(id int64, status, detail string) {
	c.actionWG.Add(1)
	select {
	case c.terminalResults <- core.AlertUpdate{ID: id, TerminalStatus: status, TerminalError: detail}:
	default:
		c.terminalResultError(errors.New("terminal result queue overflow"), 1)
		c.actionWG.Done()
	}
}

func (c *Controller) terminalResultError(err error, count int) {
	c.terminalErrors.Add(uint64(count))
	c.queueMu.Lock()
	defer c.queueMu.Unlock()
	c.queueHealth.Error = err.Error()
	if c.pipelineError == nil {
		c.pipelineError = err
	}
}
