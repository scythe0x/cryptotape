package desktop

import (
	"context"
	"cryptotape/internal/core"
	"errors"
	"fmt"
	"log/slog"
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
		if saveErr := c.store.UpdateTerminal(id, status, result.Error); saveErr != nil {
			slog.Error("save terminal status", "error", saveErr)
		}
		c.emit("alert:update", map[string]any{"id": id, "terminalStatus": status, "terminalError": result.Error})
	}
	return result
}

func (c *Controller) openAutomatically(alert core.Alert) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var err error
	status, detail := "ok", ""
	if time.Since(time.UnixMilli(alert.EventTimeMs)) > 10*time.Second {
		status = "stale"
		err = errors.New("alert is older than 10 seconds")
	} else {
		_, err = c.openInTerminals(ctx, alert.Symbol, alert.Terminals)
	}
	if err != nil {
		if status != "stale" {
			status = "error"
		}
		detail = err.Error()
	}
	if saveErr := c.store.UpdateTerminal(alert.ID, status, detail); saveErr != nil {
		slog.Error("save terminal status", "error", saveErr)
	}
	c.emit("alert:update", map[string]any{"id": alert.ID, "terminalStatus": status, "terminalError": detail})
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
	for _, terminal := range terminals {
		go func() {
			err := c.openInTerminal(ctx, symbol, terminal)
			if err != nil {
				err = fmt.Errorf("%s: %w", terminal, err)
			}
			results <- err
		}()
	}
	var failures []error
	opened := 0
	for range terminals {
		if err := <-results; err != nil {
			failures = append(failures, err)
		} else {
			opened++
		}
	}
	return opened, errors.Join(failures...)
}
