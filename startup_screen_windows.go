//go:build windows

package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
	"strconv"
)

func startupScreen(app *application.App) *application.Screen {
	x, y, ok := w32.GetCursorPos()
	if ok {
		monitor := w32.MonitorFromPoint(x, y, w32.MONITOR_DEFAULTTONEAREST)
		if screen := app.Screen.GetByID(strconv.Itoa(int(monitor))); screen != nil {
			return screen
		}
	}
	return app.Screen.GetPrimary()
}
