package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestClockBoundsRecoverDisconnectedMonitor(t *testing.T) {
	primary := application.Rect{X: 0, Y: 25, Width: 1920, Height: 1055}
	left := application.Rect{X: -1280, Y: 0, Width: 1280, Height: 1024}
	for _, test := range []struct {
		name   string
		bounds application.Rect
		areas  []application.Rect
		want   application.Rect
	}{
		{"unchanged", application.Rect{X: 50, Y: 50, Width: 270, Height: 70}, []application.Rect{primary}, application.Rect{X: 50, Y: 50, Width: 270, Height: 70}},
		{"disconnected", application.Rect{X: 2200, Y: 200, Width: 270, Height: 70}, []application.Rect{primary}, application.Rect{X: 1650, Y: 200, Width: 270, Height: 70}},
		{"negative monitor", application.Rect{X: -1000, Y: 200, Width: 270, Height: 70}, []application.Rect{primary, left}, application.Rect{X: -1000, Y: 200, Width: 270, Height: 70}},
		{"oversized", application.Rect{X: -900, Y: -200, Width: 4000, Height: 2000}, []application.Rect{primary}, primary},
		{"minimum", application.Rect{X: 40, Y: 40, Width: 1, Height: 1}, []application.Rect{primary}, application.Rect{X: 40, Y: 40, Width: clockMinWidth, Height: clockMinHeight}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := clockBounds(test.bounds, test.areas); got != test.want {
				t.Fatalf("got %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestClockStateRoundTripAndCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clock.json")
	clock := newCandleClock(nil, path)
	if clock.state.Visible || clock.state.Timezone != "system" || clock.state.Bounds.Width != 270 || clock.state.Bounds.Height != 70 {
		t.Fatalf("defaults: %+v", clock.state)
	}
	clock.state = clockState{Bounds: application.Rect{X: -800, Y: 100, Width: 540, Height: 140}, Timezone: "UTC345", Visible: true}
	if err := clock.saveLocked(); err != nil {
		t.Fatal(err)
	}
	if got := newCandleClock(nil, path).state; got != clock.state {
		t.Fatalf("got %+v, want %+v", got, clock.state)
	}
	clock.state.Visible = false
	if err := clock.saveLocked(); err != nil {
		t.Fatal(err)
	}
	if newCandleClock(nil, path).state.Visible {
		t.Fatal("closed window restored as open")
	}
	if err := os.WriteFile(path, []byte(`{"timezone":"invalid"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if newCandleClock(nil, path).state.Timezone != "system" {
		t.Fatal("invalid timezone survived")
	}
	if err := os.WriteFile(path, []byte(`{"visible":true, broken`), 0600); err != nil {
		t.Fatal(err)
	}
	if newCandleClock(nil, path).state.Visible {
		t.Fatal("corrupt state partially applied")
	}
}

func TestClockTimezoneValidation(t *testing.T) {
	for _, zone := range []string{"system", "UTC0", "UTC300", "UTC345", "UTC-210", "UTC-720", "UTC840"} {
		if !validClockTimezone(zone) {
			t.Errorf("valid timezone rejected: %s", zone)
		}
	}
	for _, zone := range []string{"", "UTC", "UTC+300", "UTC301", "UTC-735", "UTC855", "UTC300junk"} {
		if validClockTimezone(zone) {
			t.Errorf("invalid timezone accepted: %s", zone)
		}
	}
}

func TestClockWindowOptions(t *testing.T) {
	bounds := application.Rect{X: 40, Y: 50, Width: 270, Height: 70}
	options := clockOptions(bounds)
	if !options.AlwaysOnTop || !options.Frameless || options.DisableResize || options.BackgroundType != application.BackgroundTypeTransparent {
		t.Fatal("clock must be transparent, resizable and always on top")
	}
	if options.Width != 270 || options.Height != 70 || options.X != 40 || options.Y != 50 || options.InitialPosition != application.WindowXY {
		t.Fatal("initial clock geometry lost")
	}
	if !options.Mac.PanelPreferences.FloatingPanel || !options.Mac.PanelPreferences.NonActivating {
		t.Fatal("clock must float above inactive applications")
	}
}

func TestClockTimezoneWorksWithoutActivation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clock.json")
	service := &DesktopService{clock: newCandleClock(nil, path)}
	if err := service.SetClockTimezone("UTC345"); err != nil {
		t.Fatal(err)
	}
	zone, err := service.GetClockTimezone()
	if err != nil || zone != "UTC345" {
		t.Fatalf("timezone: %q, %v", zone, err)
	}
	if newCandleClock(nil, path).state.Timezone != zone {
		t.Fatal("timezone was not persisted")
	}
	if err := service.SetClockTimezone("invalid"); err == nil {
		t.Fatal("invalid timezone accepted")
	}
}
