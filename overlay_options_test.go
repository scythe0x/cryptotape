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
