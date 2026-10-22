//go:build !windows

package main

import "github.com/wailsapp/wails/v3/pkg/application"

func windowFrameSize(window *application.WebviewWindow, viewportWidth, viewportHeight int) (float64, float64, error) {
	width, height := window.Size()
	return float64(width - viewportWidth), float64(height - viewportHeight), nil
}
