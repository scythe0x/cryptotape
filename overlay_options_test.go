package main

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestOverlayUsesTransparentNonActivatingMacPanel(t *testing.T) {
	options := overlayOptions()
	if options.BackgroundType != application.BackgroundTypeTransparent || options.Mac.Backdrop != application.MacBackdropTransparent {
		t.Fatal("both native window and webview must be transparent")
	}
	if options.Mac.WindowClass != application.MacWindowClassPanel || !options.Mac.PanelPreferences.NonActivating {
		t.Fatal("notifications must not activate the app")
	}
	if options.Mac.CollectionBehavior&application.MacWindowCollectionBehaviorFullScreenAuxiliary == 0 {
		t.Fatal("notifications must be visible above fullscreen apps")
	}
}

func TestOverlayDisablesWindowsDecorations(t *testing.T) {
	options := overlayOptions()
	if !options.Windows.DisableFramelessWindowDecorations || !options.Hidden {
		t.Fatal("overlay must start hidden without native shadows")
	}
}

func TestOverlayBoundsStayAnchoredToWorkArea(t *testing.T) {
	area := application.Rect{X: -1920, Y: 40, Width: 1920, Height: 1040}
	for _, height := range []int{1, 52, 160, 300, 5000} {
		bounds := overlayBounds(area, height)
		if bounds.Width != 360 || bounds.Height != min(height, 1008) {
			t.Fatalf("unexpected measured size: %+v", bounds)
		}
		if bounds.X+bounds.Width != -16 || bounds.Y+bounds.Height != 1064 {
			t.Fatalf("overlay moved away from bottom right: %+v", bounds)
		}
	}
}
