package main

import (
	"path/filepath"
	"reflect"
	"testing"

	"cryptotape/internal/desktop"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type testNativeWindow struct {
	calls  []string
	events []SettingsSession
}

func (w *testNativeWindow) Show() application.Window   { w.calls = append(w.calls, "show"); return nil }
func (w *testNativeWindow) Hide() application.Window   { w.calls = append(w.calls, "hide"); return nil }
func (w *testNativeWindow) Focus()                     { w.calls = append(w.calls, "focus") }
func (w *testNativeWindow) SetBounds(application.Rect) {}
func (w *testNativeWindow) EmitEvent(name string, data ...any) bool {
	w.calls = append(w.calls, name)
	w.events = append(w.events, data[0].(SettingsSession))
	return false
}

func TestSettingsWindowReopensSameWindowWithFreshSession(t *testing.T) {
	window := &testNativeWindow{}
	s := &DesktopService{access: func() bool { return true }, settingsWindow: window, settingsSession: SettingsSession{Section: "alerts"}}
	for _, section := range []string{"columns", "alerts"} {
		if err := s.OpenSettings(section); err != nil {
			t.Fatal(err)
		}
		s.HideSettings()
	}
	if !reflect.DeepEqual(window.events, []SettingsSession{{"columns", 1}, {"alerts", 2}}) {
		t.Fatalf("stale settings sessions: %+v", window.events)
	}
	if !reflect.DeepEqual(window.calls, []string{"show", "focus", "settings:opened", "hide", "show", "focus", "settings:opened", "hide"}) {
		t.Fatalf("unexpected window lifecycle: %v", window.calls)
	}
	if s.GetSettingsSession() != (SettingsSession{Section: "alerts", Revision: 2}) {
		t.Fatal("opening state must be recoverable after a missed browser event")
	}
	if err := s.OpenSettings("invalid"); err == nil || len(window.events) != 2 {
		t.Fatal("unknown sections must not open a window")
	}
}

func TestNativeMinimumIncludesFrameInLogicalPixels(t *testing.T) {
	for _, test := range []struct {
		frameWidth, frameHeight float64
		width, height           int
	}{
		{16, 39, 328, 149}, {15.34, 38.67, 328, 149}, {0, 0, 312, 110}, {-1, -1, 312, 110},
	} {
		width, height := nativeMinimum(312, 110, test.frameWidth, test.frameHeight)
		if width != test.width || height != test.height {
			t.Fatalf("frame %+v: got %dx%d", test, width, height)
		}
	}
}

func TestOverlayShowsWithoutBrowserMeasurementAndCanReopen(t *testing.T) {
	controller, err := desktop.NewController(filepath.Join(t.TempDir(), "state.db"), func(string, any) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := controller.Close(); err != nil {
			t.Error(err)
		}
	})
	window := &testNativeWindow{}
	s := &DesktopService{access: func() bool { return true }, controller: controller, overlay: window}
	s.showOverlay(1)
	if !s.visible || s.overlayHeight != compactOverlayHeight || !reflect.DeepEqual(window.calls, []string{"show"}) {
		t.Fatal("native visibility must not wait for hidden webview frames")
	}
	s.ResizeOverlay(150, 1)
	s.HideOverlay(1)
	s.ResizeOverlay(150, 1)
	if s.visible || !reflect.DeepEqual(window.calls, []string{"show", "hide"}) {
		t.Fatal("late measurements must not revive dismissed notifications")
	}
	s.showOverlay(2)
	if !s.visible || s.overlayHeight != compactOverlayHeight {
		t.Fatal("next notification must show with compact fallback bounds")
	}
	s.HideOverlay(1)
	s.ResizeOverlay(200, 1)
	if !s.visible || s.overlayHeight != compactOverlayHeight {
		t.Fatal("old hide/resize must not affect the new notification")
	}
	s.ResizeOverlay(110, 2)
	if s.overlayHeight != 110 || !reflect.DeepEqual(window.calls, []string{"show", "hide", "show"}) {
		t.Fatal("browser measurements must refine height without extra show operations")
	}
}

func TestDesktopRPCsRejectAccessBeforeControllerUse(t *testing.T) {
	s := &DesktopService{}
	if _, err := s.GetSnapshot(); err == nil {
		t.Fatal("snapshot was accessible")
	}
	if err := s.OpenSettings("alerts"); err == nil {
		t.Fatal("settings were accessible")
	}
	if err := s.OpenTerminal(0, "BTCUSDT", "vataga"); err == nil {
		t.Fatal("terminal was accessible")
	}
	if s.SoundIDs() != nil || s.Monitors() != nil {
		t.Fatal("native data was accessible")
	}
	if result := s.OpenTerminals(0, "BTCUSDT", []string{"vataga"}); result.Error == "" {
		t.Fatal("terminal batch was accessible")
	}
}
