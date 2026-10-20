package main

import (
	"context"
	"embed"
	"log"
	"log/slog"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"cryptotape/internal/core"
	"cryptotape/internal/desktop"
	"cryptotape/internal/storage"
)

//go:embed frontend/dist
var assets embed.FS

type Monitor struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Primary bool   `json:"primary"`
}

type DesktopService struct {
	controller *desktop.Controller
	app        *application.App
	overlay    *application.WebviewWindow
	overlayMu  sync.Mutex
	visible    bool
	latestID   int64
}

func (s *DesktopService) GetSnapshot() (core.Snapshot, error) {
	return s.controller.GetSnapshot()
}

func (s *DesktopService) SaveRule(rule core.Rule) (core.Rule, error) {
	return s.controller.SaveRule(rule)
}

func (s *DesktopService) DeleteRule(id int64) error {
	return s.controller.DeleteRule(id)
}

func (s *DesktopService) SaveSettings(settings core.Settings) error {
	if err := s.controller.SaveSettings(settings); err != nil {
		return err
	}
	s.placeOverlay(settings.MonitorID)
	return nil
}

func (s *DesktopService) TestProxy(proxy string) error {
	return s.controller.TestProxy(proxy)
}

func (s *DesktopService) ListAlerts(before int64, limit int) ([]core.Alert, error) {
	return s.controller.ListAlerts(before, limit)
}

func (s *DesktopService) AlertsAfter(after int64, limit int) ([]core.Alert, error) {
	return s.controller.AlertsAfter(after, limit)
}

func (s *DesktopService) SessionAlerts() ([]core.Alert, error) {
	return s.controller.SessionAlerts()
}

func (s *DesktopService) OpenTerminal(id int64, symbol string, terminal string) error {
	return s.controller.OpenTerminal(id, symbol, terminal)
}

func (s *DesktopService) OpenTerminals(id int64, symbol string, terminals []string) desktop.OpenResult {
	return s.controller.OpenTerminals(id, symbol, terminals)
}

func (s *DesktopService) SoundIDs() []string { return s.controller.SoundIDs() }

func (s *DesktopService) ImportSound(name, extension, encoded string) (string, error) {
	return s.controller.ImportSound(name, extension, encoded)
}

func (s *DesktopService) PreviewSound(id string, volume float64) error {
	return s.controller.PreviewSound(id, volume)
}

func (s *DesktopService) HideOverlay(afterID int64) {
	s.overlayMu.Lock()
	defer s.overlayMu.Unlock()
	if afterID < s.latestID {
		return
	}
	s.overlay.Hide()
	s.visible = false
}

func (s *DesktopService) showOverlay(id int64) {
	s.overlayMu.Lock()
	defer s.overlayMu.Unlock()
	s.latestID = max(s.latestID, id)
	if !s.visible {
		s.overlay.Show()
		if snapshot, err := s.controller.GetSnapshot(); err == nil {
			s.placeOverlay(snapshot.Settings.MonitorID)
		} else {
			slog.Warn("overlay position", "error", err)
			s.placeOverlay("")
		}
		s.visible = true
	}
}

func (s *DesktopService) Monitors() []Monitor {
	screens := s.app.Screen.GetAll()
	monitors := make([]Monitor, 0, len(screens))
	for _, screen := range screens {
		monitors = append(monitors, Monitor{ID: screen.ID, Name: screen.Name, Primary: screen.IsPrimary})
	}
	return monitors
}

func (s *DesktopService) placeOverlay(monitorID string) {
	if s.app == nil || s.overlay == nil {
		return
	}
	screen := s.app.Screen.GetByID(monitorID)
	if screen == nil {
		screen = s.app.Screen.GetPrimary()
	}
	if screen == nil {
		return
	}
	const width, height, margin = 360, 300, 16
	area := screen.WorkArea
	s.overlay.SetPosition(area.X+area.Width-width-margin, area.Y+area.Height-height-margin)
}

func main() {
	path, err := storage.DefaultPath()
	if err != nil {
		log.Fatal(err)
	}
	service := &DesktopService{}
	controller, err := desktop.NewController(path, func(name string, data any) {
		if alert, ok := data.(core.Alert); name == "alert:new" && ok && service.overlay != nil {
			service.showOverlay(alert.ID)
		}
		if service.app != nil {
			service.app.Event.Emit(name, data)
		}
	})
	if err != nil {
		log.Fatal(err)
	}
	service.controller = controller
	app := application.New(application.Options{
		Name: "cryptotape", Description: "fast Binance futures alerts",
		Assets:   application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Services: []application.Service{application.NewService(service)},
		Mac:      application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	service.app = app
	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "main", Title: "cryptotape", Width: 440, Height: 740,
		MinWidth: 420, MinHeight: 420, URL: "/",
	})
	service.overlay = app.Window.NewWithOptions(overlayOptions())
	mainWindow.RegisterHook(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		app.Quit()
	})
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) {
		if err := makeNonActivating(service.overlay); err != nil {
			slog.Warn("overlay focus style", "error", err)
		}
		snapshot, err := controller.GetSnapshot()
		if err == nil {
			service.placeOverlay(snapshot.Settings.MonitorID)
		}
		controller.Start(app.Context())
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
	if err := controller.Close(); err != nil && err != context.Canceled {
		log.Print(err)
	}
}
