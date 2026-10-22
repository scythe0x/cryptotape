package main

import (
	"context"
	"embed"
	"log"
	"log/slog"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"cryptotape/internal/core"
	"cryptotape/internal/desktop"
	"cryptotape/internal/storage"
)

//go:embed frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func main() {
	path, err := storage.DefaultPath()
	if err != nil {
		log.Fatal(err)
	}
	service := &DesktopService{settingsSession: SettingsSession{Section: "alerts"}}
	service.access = func() bool { return runtime.GOOS != "windows" }
	activation, err := newActivation(service, path)
	if err != nil {
		log.Fatal(err)
	}
	service.activation = activation
	controller, err := desktop.NewController(path, func(name string, data any) {
		if !service.access() {
			return
		}
		if alert, ok := data.(core.Alert); name == "alert:new" && ok && alert.BannerSeconds != 0 && service.overlay != nil {
			service.showOverlay(alert.ID)
		}
		if settings, ok := data.(core.Settings); name == "settings:changed" && ok && !settings.BannersEnabled && service.overlay != nil {
			service.overlayMu.Lock()
			service.overlay.Hide()
			service.visible = false
			service.hiddenThroughID = max(service.hiddenThroughID, service.latestID)
			service.overlayMu.Unlock()
		}
		if service.app != nil {
			service.app.Event.Emit(name, data)
		}
	})
	if err != nil {
		log.Fatal(err)
	}
	service.controller = controller
	controller.SetAccessCheck(service.access)
	app := application.New(application.Options{
		Name: "Mythra", Icon: appIcon, Description: "fast Binance futures alerts",
		Assets:   application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Services: []application.Service{application.NewService(service), application.NewService(activation)},
		Mac:      application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	service.app = app
	if runtime.GOOS == "windows" {
		activation.createWindow(app)
	}
	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "main", Title: "Mythra", Width: 440, Height: 740, Hidden: runtime.GOOS == "windows",
		URL: "/",
	})
	service.mainWindow = mainWindow
	settingsWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "settings", Title: "Mythra settings", Width: 520, Height: 720,
		MinWidth: 360, MinHeight: 240, Hidden: true, URL: "/?settings=1",
	})
	service.settingsWindow = settingsWindow
	settingsWindow.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		service.HideSettings()
		event.Cancel()
	})
	overlay := app.Window.NewWithOptions(overlayOptions())
	service.overlay = overlay
	mainWindow.RegisterHook(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		app.Quit()
	})
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) {
		if err := makeNonActivating(overlay); err != nil {
			slog.Warn("overlay focus style", "error", err)
		}
		snapshot, err := controller.GetSnapshot()
		if err == nil {
			service.placeOverlay(snapshot.Settings.MonitorID)
		}
		if runtime.GOOS != "windows" {
			controller.Start(app.Context())
		}
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
	if err := controller.Close(); err != nil && err != context.Canceled {
		log.Print(err)
	}
}
