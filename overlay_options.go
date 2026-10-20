package main

import "github.com/wailsapp/wails/v3/pkg/application"

func overlayOptions() application.WebviewWindowOptions {
	return application.WebviewWindowOptions{
		Name: "overlay", Title: "cryptotape alerts", Width: 360, Height: 300,
		URL: "/?overlay=1", Hidden: true, Frameless: true, DisableResize: true,
		AlwaysOnTop: true, BackgroundType: application.BackgroundTypeTransparent,
		Mac: application.MacWindow{
			Backdrop:      application.MacBackdropTransparent,
			DisableShadow: true,
			WindowClass:   application.MacWindowClassPanel,
			PanelPreferences: application.MacPanelPreferences{
				NonActivating: true, FloatingPanel: true, BecomesKeyOnlyIfNeeded: true,
			},
			CollectionBehavior: application.MacWindowCollectionBehaviorCanJoinAllSpaces |
				application.MacWindowCollectionBehaviorFullScreenAuxiliary |
				application.MacWindowCollectionBehaviorIgnoresCycle,
		},
	}
}
