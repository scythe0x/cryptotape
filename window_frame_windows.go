//go:build windows

package main

import (
	"fmt"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
)

type windowRect struct{ Left, Top, Right, Bottom int32 }

func windowFrameSize(window *application.WebviewWindow, _, _ int) (float64, float64, error) {
	handle := uintptr(unsafe.Pointer(window.NativeWindow()))
	user32 := windows.NewLazySystemDLL("user32.dll")
	var outer, client windowRect
	for _, query := range []struct {
		name string
		rect *windowRect
	}{
		{"GetWindowRect", &outer},
		{"GetClientRect", &client},
	} {
		ok, _, err := user32.NewProc(query.name).Call(handle, uintptr(unsafe.Pointer(query.rect)))
		if ok == 0 {
			return 0, 0, fmt.Errorf("window geometry: %s: %w", query.name, err)
		}
	}
	dpi, _, err := user32.NewProc("GetDpiForWindow").Call(handle)
	if dpi == 0 {
		return 0, 0, fmt.Errorf("window geometry: GetDpiForWindow: %w", err)
	}
	// both rectangles are physical pixels; window constraints use logical pixels.
	scale := 96 / float64(dpi)
	return float64(outer.Right-outer.Left-client.Right+client.Left) * scale,
		float64(outer.Bottom-outer.Top-client.Bottom+client.Top) * scale, nil
}
