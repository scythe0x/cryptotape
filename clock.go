package main

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const clockMinWidth, clockMinHeight = 180, 60

type clockState struct {
	Bounds   application.Rect `json:"bounds"`
	Timezone string           `json:"timezone"`
	Visible  bool             `json:"visible"`
}

type candleClock struct {
	mu       sync.Mutex
	app      *application.App
	window   *application.WebviewWindow
	path     string
	state    clockState
	quitting bool
	changed  chan struct{}
}

func clockOptions(bounds application.Rect) application.WebviewWindowOptions {
	options := overlayOptions()
	options.Name, options.Title, options.URL = "clock", "Mythra clock", "/?clock=1"
	options.X, options.Y, options.Width, options.Height = bounds.X, bounds.Y, bounds.Width, bounds.Height
	options.MinWidth, options.MinHeight = clockMinWidth, clockMinHeight
	options.InitialPosition = application.WindowXY
	options.DisableResize = false
	options.Mac.CornerType = application.MacWindowCornerTypeSquare
	return options
}

func (c *candleClock) areas() []application.Rect {
	screens := c.app.Screen.GetAll()
	areas := make([]application.Rect, 0, len(screens))
	for _, screen := range screens {
		if screen.WorkArea.Width > 0 && screen.WorkArea.Height > 0 {
			areas = append(areas, screen.WorkArea)
		}
	}
	return areas
}

func (c *candleClock) open() error {
	areas := c.areas()
	c.mu.Lock()
	if c.window != nil {
		window := c.window
		c.mu.Unlock()
		window.Show()
		window.Focus()
		return nil
	}
	c.state.Bounds = clockBounds(c.state.Bounds, areas)
	bounds := c.state.Bounds
	window := c.app.Window.NewWithOptions(clockOptions(bounds))
	c.window = window
	c.state.Visible = true
	err := c.saveLocked()
	c.mu.Unlock()
	window.RegisterHook(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		bounds := window.Bounds()
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.window != window {
			return
		}
		c.state.Bounds = bounds
		if !c.quitting {
			c.state.Visible = false
		}
		c.window = nil
		c.reportSaveLocked()
	})
	for _, event := range []events.WindowEventType{events.Common.WindowDidMove, events.Common.WindowDidResize} {
		window.OnWindowEvent(event, func(_ *application.WindowEvent) {
			select {
			case c.changed <- struct{}{}:
			default:
			}
		})
	}
	window.Show()
	window.SetBounds(bounds)
	return err
}

func (c *candleClock) capture() {
	c.mu.Lock()
	window := c.window
	c.mu.Unlock()
	if window == nil {
		return
	}
	bounds := window.Bounds()
	visible := clockBounds(bounds, c.areas())
	if visible != bounds {
		window.SetBounds(visible)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.window == window && c.state.Bounds != visible {
		c.state.Bounds = visible
		c.reportSaveLocked()
	}
}

func (c *candleClock) start(ctx context.Context, allowed func() bool) {
	c.mu.Lock()
	restore := c.state.Visible
	c.mu.Unlock()
	if restore && allowed() {
		if err := c.open(); err != nil {
			slog.Warn("restore clock", "error", err)
		}
	}
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		debounce := time.NewTimer(time.Hour)
		debounce.Stop()
		defer debounce.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.changed:
				debounce.Reset(250 * time.Millisecond)
			case <-debounce.C:
				c.capture()
			case <-ticker.C:
				c.capture()
			}
		}
	}()
}

func (s *DesktopService) OpenClock() error {
	if err := s.requireAccess(); err != nil {
		return err
	}
	if s.clock == nil {
		return errors.New("clock unavailable")
	}
	return s.clock.open()
}

func (s *DesktopService) GetClockTimezone() (string, error) {
	if err := s.requireAccess(); err != nil {
		return "", err
	}
	s.clock.mu.Lock()
	defer s.clock.mu.Unlock()
	return s.clock.state.Timezone, nil
}

func (s *DesktopService) SetClockTimezone(zone string) error {
	if err := s.requireAccess(); err != nil {
		return err
	}
	if !validClockTimezone(zone) {
		return errors.New("invalid clock timezone")
	}
	s.clock.mu.Lock()
	defer s.clock.mu.Unlock()
	previous := s.clock.state.Timezone
	s.clock.state.Timezone = zone
	if err := s.clock.saveLocked(); err != nil {
		s.clock.state.Timezone = previous
		return err
	}
	return nil
}

func (c *candleClock) restore() {
	c.mu.Lock()
	visible := c.state.Visible
	c.mu.Unlock()
	if visible {
		if err := c.open(); err != nil {
			slog.Warn("restore clock", "error", err)
		}
	}
}

func (c *candleClock) hide() {
	c.mu.Lock()
	window := c.window
	c.mu.Unlock()
	if window != nil {
		window.Hide()
	}
}

func (c *candleClock) prepareQuit() {
	c.capture()
	c.mu.Lock()
	c.quitting = true
	c.reportSaveLocked()
	c.mu.Unlock()
}
