package main

import "github.com/wailsapp/wails/v3/pkg/application"

const compactOverlayHeight = 52

type overlayWindow interface {
	Show() application.Window
	Hide() application.Window
	SetBounds(application.Rect)
}

func (s *DesktopService) HideOverlay(afterID int64) {
	s.overlayMu.Lock()
	defer s.overlayMu.Unlock()
	if afterID < s.latestID || s.overlay == nil {
		return
	}
	s.overlay.Hide()
	s.visible = false
	s.hiddenThroughID = max(s.hiddenThroughID, afterID)
}

func (s *DesktopService) showOverlay(id int64) {
	s.overlayMu.Lock()
	defer s.overlayMu.Unlock()
	if !s.controller.BannersEnabled() || s.overlay == nil || id <= s.hiddenThroughID {
		return
	}
	s.latestID = max(s.latestID, id)
	if !s.visible {
		s.overlayHeight = compactOverlayHeight
		s.placeOverlay(s.controller.MonitorID())
		s.overlay.Show()
		s.visible = true
	}
}

func (s *DesktopService) ResizeOverlay(height int, afterID int64) {
	s.overlayMu.Lock()
	defer s.overlayMu.Unlock()
	if height <= 0 || afterID < s.latestID || afterID <= s.hiddenThroughID || s.overlay == nil || !s.controller.BannersEnabled() {
		return
	}
	s.latestID = max(s.latestID, afterID)
	s.overlayHeight = height
	s.placeOverlay(s.controller.MonitorID())
	if !s.visible {
		s.overlay.Show()
		s.visible = true
	}
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
	s.overlay.SetBounds(overlayBounds(screen.WorkArea, s.overlayHeight))
}

func overlayBounds(area application.Rect, height int) application.Rect {
	const width, margin = 360, 16
	height = max(1, min(height, area.Height-2*margin))
	return application.Rect{X: area.X + area.Width - width - margin, Y: area.Y + area.Height - height - margin, Width: width, Height: height}
}
