package main

import (
	"cryptotape/internal/core"
	"cryptotape/internal/desktop"
	"github.com/wailsapp/wails/v3/pkg/application"
	"sync"
)

type Monitor struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Primary bool   `json:"primary"`
}

type DesktopService struct {
	clock           *candleClock
	controller      *desktop.Controller
	app             *application.App
	mainWindow      *application.WebviewWindow
	settingsWindow  settingsNativeWindow
	settingsMu      sync.Mutex
	settingsSession SettingsSession
	minimumMu       sync.Mutex
	minimumWidth    int
	minimumHeight   int
	overlay         overlayWindow
	overlayMu       sync.Mutex
	visible         bool
	latestID        int64
	hiddenThroughID int64
	overlayHeight   int
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

func (s *DesktopService) SetSoundMuted(muted bool) error {
	return s.controller.SetSoundMuted(muted)
}

func (s *DesktopService) SaveSettings(settings core.Settings) error {
	if err := s.controller.SaveSettings(settings); err != nil {
		return err
	}
	s.overlayMu.Lock()
	s.placeOverlay(settings.MonitorID)
	s.overlayMu.Unlock()
	return nil
}

func (s *DesktopService) SetTableColumns(columns []core.TableColumnID) error {
	return s.controller.SetTableColumns(columns)
}

func (s *DesktopService) SaveProxy(profile core.ProxyProfile, use bool) error {
	return s.controller.SaveProxy(profile, use)
}

func (s *DesktopService) SelectProxy(id string) error {
	return s.controller.SelectProxy(id)
}

func (s *DesktopService) DeleteProxy(id string) error {
	return s.controller.DeleteProxy(id)
}

func (s *DesktopService) Reconnect() {
	s.controller.Reconnect()
}

func (s *DesktopService) ListingSource() desktop.ListingSource {
	return s.controller.ListingSource()
}

func (s *DesktopService) SaveListingSource(endpoint, key string) error {
	return s.controller.SaveListingSource(endpoint, key)
}

func (s *DesktopService) ClearListingKey() error {
	return s.controller.ClearListingKey()
}

func (s *DesktopService) ListAlerts(before int64, limit int) ([]core.Alert, error) {
	return s.controller.ListAlerts(before, limit)
}

func (s *DesktopService) AlertsAfter(after int64, limit int) ([]core.Alert, error) {
	return s.controller.AlertsAfter(after, limit)
}

func (s *DesktopService) AlertRendered(id int64, view string) {
	s.controller.AlertRendered(id, view)
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

func (s *DesktopService) SoundIDs() []string {
	return s.controller.SoundIDs()
}

func (s *DesktopService) ImportSound(name, extension, encoded string) (string, error) {
	return s.controller.ImportSound(name, extension, encoded)
}

func (s *DesktopService) PreviewSound(id string, volume float64) error {
	return s.controller.PreviewSound(id, volume)
}

func (s *DesktopService) Monitors() []Monitor {
	screens := s.app.Screen.GetAll()
	monitors := make([]Monitor, 0, len(screens))
	for _, screen := range screens {
		monitors = append(monitors, Monitor{ID: screen.ID, Name: screen.Name, Primary: screen.IsPrimary})
	}
	return monitors
}
