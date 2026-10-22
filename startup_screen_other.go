//go:build !windows

package main

import "github.com/wailsapp/wails/v3/pkg/application"

func startupScreen(app *application.App) *application.Screen { return app.Screen.GetPrimary() }
