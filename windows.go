package main

import (
	"errors"
	"math"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type settingsNativeWindow interface {
	Show() application.Window
	Hide() application.Window
	Focus()
	EmitEvent(string, ...any) bool
}

type SettingsSession struct {
	Section  string `json:"section"`
	Revision uint64 `json:"revision"`
}

func validSettingsSection(section string) bool {
	switch section {
	case "alerts", "proxies", "listings", "general", "columns":
		return true
	default:
		return false
	}
}

func (s *DesktopService) OpenSettings(section string) error {

	if !validSettingsSection(section) {
		return errors.New("invalid settings section")
	}
	if s.settingsWindow == nil {
		return errors.New("settings window unavailable")
	}
	s.settingsMu.Lock()
	s.settingsSession.Section = section
	s.settingsSession.Revision++
	session := s.settingsSession
	s.settingsMu.Unlock()

	s.settingsWindow.Show()
	s.settingsWindow.Focus()
	s.settingsWindow.EmitEvent("settings:opened", session)
	return nil
}

func (s *DesktopService) HideSettings() {
	if s.settingsWindow != nil {
		s.settingsWindow.Hide()
	}
}

func (s *DesktopService) GetSettingsSession() SettingsSession {

	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	return s.settingsSession
}

func (s *DesktopService) SetMainWindowMinimum(width, height, viewportWidth, viewportHeight int) error {

	if width <= 0 || height <= 0 || viewportWidth <= 0 || viewportHeight <= 0 ||
		width > 65536 || height > 65536 || viewportWidth > 65536 || viewportHeight > 65536 {
		return errors.New("invalid window minimum")
	}
	if s.mainWindow == nil {
		return errors.New("main window unavailable")
	}

	s.minimumMu.Lock()
	defer s.minimumMu.Unlock()
	frameWidth, frameHeight, err := windowFrameSize(s.mainWindow, viewportWidth, viewportHeight)
	if err != nil {
		return err
	}
	minimumWidth, minimumHeight := nativeMinimum(width, height, frameWidth, frameHeight)
	if minimumWidth == s.minimumWidth && minimumHeight == s.minimumHeight {
		return nil
	}
	s.mainWindow.SetMinSize(minimumWidth, minimumHeight)
	s.minimumWidth, s.minimumHeight = minimumWidth, minimumHeight
	return nil
}

func nativeMinimum(width, height int, frameWidth, frameHeight float64) (int, int) {
	return width + int(math.Ceil(math.Max(0, frameWidth))), height + int(math.Ceil(math.Max(0, frameHeight)))
}
