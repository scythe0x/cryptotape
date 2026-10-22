package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func newCandleClock(app *application.App, path string) *candleClock {
	c := &candleClock{
		app: app, path: path, changed: make(chan struct{}, 1),
		state: clockState{
			Bounds:   application.Rect{X: 40, Y: 40, Width: 270, Height: 70},
			Timezone: "system",
		},
	}
	data, err := os.ReadFile(path)
	if err == nil {
		loaded := c.state
		if err := json.Unmarshal(data, &loaded); err != nil {
			slog.Warn("read clock state", "error", err)
		} else {
			c.state = loaded
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		slog.Warn("read clock state", "error", err)
	}
	if !validClockTimezone(c.state.Timezone) {
		c.state.Timezone = "system"
	}
	return c
}

func validClockTimezone(zone string) bool {
	if zone == "system" {
		return true
	}
	var minutes int
	if _, err := fmt.Sscanf(zone, "UTC%d", &minutes); err != nil {
		return false
	}
	return minutes >= -720 && minutes <= 840 && minutes%15 == 0 && zone == fmt.Sprintf("UTC%d", minutes)
}

// keep the whole window inside the closest available work area, including negative monitor coordinates.
func clockBounds(bounds application.Rect, areas []application.Rect) application.Rect {
	if len(areas) == 0 {
		return bounds
	}
	best := areas[0]
	distance := int64(1 << 62)
	for _, area := range areas {
		x := max(area.X, min(bounds.X, area.X+area.Width-min(bounds.Width, area.Width)))
		y := max(area.Y, min(bounds.Y, area.Y+area.Height-min(bounds.Height, area.Height)))
		dx, dy := int64(x-bounds.X), int64(y-bounds.Y)
		if d := dx*dx + dy*dy; d < distance {
			best, distance = area, d
		}
	}
	bounds.Width = min(max(clockMinWidth, bounds.Width), best.Width)
	bounds.Height = min(max(clockMinHeight, bounds.Height), best.Height)
	bounds.X = max(best.X, min(bounds.X, best.X+best.Width-bounds.Width))
	bounds.Y = max(best.Y, min(bounds.Y, best.Y+best.Height-bounds.Height))
	return bounds
}

func (c *candleClock) saveLocked() error {
	data, err := json.Marshal(c.state)
	if err != nil {
		return err
	}
	if err = os.WriteFile(c.path+".tmp", data, 0600); err != nil {
		return err
	}
	return os.Rename(c.path+".tmp", c.path)
}

func (c *candleClock) reportSaveLocked() {
	if err := c.saveLocked(); err != nil {
		slog.Warn("save clock state", "error", err)
	}
}
