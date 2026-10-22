package desktop

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"cryptotape/internal/core"
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
	if c.closing || !c.actionsAllowed() {
		c.pipelineMu.RUnlock()
		return OpenResult{Error: "controller closed"}
	}
	c.actionWG.Add(1)
	c.pipelineMu.RUnlock()
	defer c.actionWG.Done()

	ctx, cancel := context.WithTimeout(c.actionContext(), 2*time.Second)
	defer cancel()
	exchanges := core.DefaultOpenExchanges()
	var ruleID int64
	if id > 0 {
		alerts, err := c.store.ListAlerts(ctx, id+1, 1)
		if err != nil {
			return OpenResult{Error: err.Error()}
		}
		if len(alerts) != 1 || alerts[0].ID != id {
			return OpenResult{Error: "alert not found"}
		}
		ruleID = alerts[0].RuleID
		exchanges = alerts[0].OpenExchanges
		symbol = alerts[0].Symbol
	}
	opened, err := c.openInTerminals(ctx, symbol, terminals, ruleID, exchanges)
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

func (c *Controller) openInTerminal(ctx context.Context, symbol, terminal string, exchanges ...string) error {
	if !c.actionsAllowed() || ctx.Err() != nil {
		return context.Canceled
	}
	if len(exchanges) == 0 {
		return errors.New("no enabled exchanges")
	}
	var unavailable []error
	for _, exchange := range exchanges {
		if !c.actionsAllowed() || ctx.Err() != nil {
			return context.Canceled
		}
		var err error
		switch terminal {
		case "vataga":
			err = c.terminal.OpenExchange(ctx, symbol, exchange)
		case "tiger":
			c.mu.RLock()
			settings := c.settings
			c.mu.RUnlock()
			err = c.tiger.OpenExchange(ctx, symbol, exchange, settings.TigerPort, settings.TigerLinkGroup)
		default:
			return errors.New("unknown terminal")
		}
		if err == nil {
			return nil
		}
		if !errors.Is(err, errSymbolUnavailable) {
			return fmt.Errorf("%s: %w", exchange, err)
		}
		unavailable = append(unavailable, fmt.Errorf("%s: %w", exchange, err))
	}
	return errors.Join(unavailable...)
}

func (c *Controller) openInTerminals(ctx context.Context, symbol string, terminals []string, ruleID int64, exchanges []string) (int, error) {
	if len(c.currentOpenExchanges(ruleID, exchanges)) == 0 {
		return 0, errors.New("no enabled exchanges")
	}
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
	if c.closing || !c.actionsAllowed() {
		c.pipelineMu.RUnlock()
		return 0, errors.New("controller closed")
	}
	for _, terminal := range terminals {
		c.enqueueTerminal(terminalJob{symbol: symbol, ruleID: ruleID, exchanges: exchanges, terminal: terminal, queued: time.Now(), expires: deadline,
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

func (c *Controller) currentOpenExchanges(ruleID int64, saved []string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, rule := range c.rules {
		if rule.ID == ruleID {
			return slices.Clone(rule.OpenExchanges)
		}
	}
	return slices.Clone(saved)
}
