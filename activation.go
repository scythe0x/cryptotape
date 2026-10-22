package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"cryptotape/internal/licensing"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

type ActivationService struct {
	desktop *DesktopService
	manager *licensing.Manager
	window  *application.WebviewWindow
	once    sync.Once
	mu      sync.Mutex
	shown   time.Time
	running bool
}

func (s *DesktopService) requireAccess() error {
	if s.access == nil || !s.access() {
		return errors.New("доступ к приложению закрыт")
	}
	return nil
}

func newActivation(desktop *DesktopService, database string) (*ActivationService, error) {
	a := &ActivationService{desktop: desktop}
	if runtime.GOOS != "windows" {
		return a, nil
	}
	key, err := base64.StdEncoding.DecodeString(licensing.PublicKeyBase64)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return nil, errors.New("release license public key is missing")
	}
	a.manager = &licensing.Manager{
		Client:   &licensing.Client{PublicKey: ed25519.PublicKey(key)},
		Hardware: licensing.Hardware,
		Keys:     licensing.FileKeys{Path: filepath.Join(filepath.Dir(database), "license.dat")},
	}
	desktop.access = a.manager.Allowed
	a.manager.Changed = a.changed
	return a, nil
}

func (a *ActivationService) createWindow(app *application.App) {
	width, height := 840, 473
	screen := startupScreen(app)
	if screen != nil {
		scale := min(1.0, float64(screen.WorkArea.Width)*0.9/float64(width), float64(screen.WorkArea.Height)*0.9/float64(height))
		width, height = int(float64(width)*scale), int(float64(height)*scale)
	}
	a.window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "activation", Title: "Mythra", Width: width, Height: height, URL: "/?activation=1",
		Frameless: true, DisableResize: true, InitialPosition: application.WindowCentered, Screen: screen,
		BackgroundColour: application.NewRGB(0, 0, 0),
		Windows:          application.WindowsWindow{DisableFramelessWindowDecorations: true},
		Mac:              application.MacWindow{DisableShadow: true},
	})
	a.window.RegisterHook(events.Common.WindowClosing, func(_ *application.WindowEvent) { app.Quit() })
}

func (a *ActivationService) State() licensing.State {
	if a.manager == nil {
		return licensing.State{Phase: "ready", BotURL: licensing.BotURL}
	}
	return a.manager.State()
}

func (a *ActivationService) FirstPaint() {
	if a.manager == nil {
		return
	}
	a.once.Do(func() {
		a.mu.Lock()
		a.shown = time.Now()
		a.mu.Unlock()
		go a.manager.Start(a.desktop.app.Context())
	})
}

func (a *ActivationService) Activate(key string) licensing.State {
	if a.manager == nil {
		return a.State()
	}
	if len(key) > 128 {
		return licensing.State{Phase: "locked", Code: "INVALID_KEY", Message: licensing.Message("INVALID_KEY"), BotURL: licensing.BotURL}
	}
	return a.manager.Authenticate(a.desktop.app.Context(), key, true)
}

func (a *ActivationService) Retry() licensing.State {
	if a.manager == nil {
		return a.State()
	}
	return a.manager.Retry(a.desktop.app.Context())
}

func (a *ActivationService) Continue() error {
	if a.manager == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.shown.IsZero() {
		return errors.New("startup has not been shown")
	}
	if remaining := time.Until(a.shown.Add(3 * time.Second)); remaining > 0 {
		timer := time.NewTimer(remaining)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-a.desktop.app.Context().Done():
			return context.Canceled
		}
	}
	s := a.desktop
	s.runMu.Lock()
	defer s.runMu.Unlock()
	if err := s.requireAccess(); err != nil {
		return err
	}
	if !a.running {
		s.controller.Start(s.app.Context())
		a.running = true
	}
	s.mainWindow.Show()
	s.mainWindow.Focus()
	a.window.Hide()
	return nil
}

func (a *ActivationService) Quit() {
	if a.desktop.app != nil {
		a.desktop.app.Quit()
	}
}

func (a *ActivationService) changed(state licensing.State) {
	s := a.desktop
	if s.app != nil {
		s.app.Event.Emit("license:state", state)
	}
	if state.Phase == "locked" {
		s.controller.Interrupt()
		// closing access happens in the manager before this callback; hide every useful window immediately.
		if s.mainWindow != nil {
			s.mainWindow.Hide()
		}
		s.HideSettings()
		s.overlayMu.Lock()
		if s.overlay != nil {
			s.overlay.Hide()
		}
		s.visible = false
		s.overlayMu.Unlock()
		if a.window != nil {
			a.window.Show()
			a.window.Focus()
		}
		a.mu.Lock()
		s.runMu.Lock()
		if a.running {
			s.controller.Suspend()
			a.running = false
		}
		s.runMu.Unlock()
		a.mu.Unlock()
	}
}
