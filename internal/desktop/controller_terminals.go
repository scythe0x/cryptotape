package desktop

import (
	"context"
	"errors"
	"fmt"

	"time"
)

type OpenResult struct {
	Opened int    `json:"opened"`
	Error  string `json:"error"`
}

func (c *Controller) OpenTerminal(id int64, symbol, terminal string) error {
	result := c.OpenTerminals(id, symbol, []string{terminal})
	if result.Error != "" {
		return errors.New(result.Error)
	}
	return nil
}

func (c *Controller) OpenTerminals(id int64, symbol string, terminals []string) OpenResult {
	c.pipelineMu.RLock()
	if c.closing {
		c.pipelineMu.RUnlock()
		return OpenResult{Error: "controller closed"}
	}
	c.actionWG.Add(1)
	c.pipelineMu.RUnlock()
	defer c.actionWG.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	opened, err := c.openInTerminals(ctx, symbol, terminals)
	result := OpenResult{Opened: opened}
	status := "manual-ok"
	if err != nil {
		status = "manual-error"
		result.Error = err.Error()
	}
	if id > 0 {
		c.saveTerminalResult(id, status, result.Error)
	}

	return result
}

func (c *Controller) openInTerminal(ctx context.Context, symbol, terminal string) error {
	switch terminal {
	case "vataga":
		return c.terminal.Open(ctx, symbol)
	case "tiger":
		c.mu.RLock()
		settings := c.settings
		c.mu.RUnlock()
		return c.tiger.Open(ctx, symbol, settings.TigerPort, settings.TigerLinkGroup)
	default:
		return errors.New("unknown terminal")
	}
}
func (c *Controller) openInTerminals(ctx context.Context, symbol string, terminals []string) (int, error) {
	if len(terminals) == 0 || len(terminals) > 2 {
		return 0, errors.New("choose a terminal")
	}
	seen := make(map[string]bool, len(terminals))
	for _, terminal := range terminals {
		if (terminal != "vataga" && terminal != "tiger") || seen[terminal] {
			return 0, errors.New("invalid terminal selection")
		}
		seen[terminal] = true
	}
	results := make(chan error, len(terminals))
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(2 * time.Second)
	}
	c.pipelineMu.RLock()
	if c.closing {
		c.pipelineMu.RUnlock()
		return 0, errors.New("controller closed")
	}
	for _, terminal := range terminals {
		c.enqueueTerminal(terminalJob{symbol: symbol, terminal: terminal, queued: time.Now(), expires: deadline,
			complete: func(err error, _ string) {
				if err != nil {
					err = fmt.Errorf("%s: %w", terminal, err)
				}
				results <- err
			},
		})
	}
	c.pipelineMu.RUnlock()
	var failures []error
	opened := 0
	for range terminals {
		select {
		case err := <-results:
			if err != nil {
				failures = append(failures, err)
			} else {
				opened++
			}
		case <-ctx.Done():
			return opened, errors.Join(append(failures, ctx.Err())...)
		}
	}
	return opened, errors.Join(failures...)
}
