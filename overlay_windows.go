//go:build windows

package main

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	gwlExStyle     = -20
	wsExNoActivate = 0x08000000
	wsExToolWindow = 0x00000080
)

func makeNonActivating(window *application.WebviewWindow) error {
	handle := uintptr(unsafe.Pointer(window.NativeWindow()))
	if handle == 0 {
		return errors.New("overlay window handle is unavailable")
	}
	user32 := windows.NewLazySystemDLL("user32.dll")
	getStyle := user32.NewProc("GetWindowLongPtrW")
	setStyle := user32.NewProc("SetWindowLongPtrW")
	index := int32(gwlExStyle)
	style, _, err := getStyle.Call(handle, uintptr(index))
	if style == 0 && err != windows.ERROR_SUCCESS {
		return err
	}
	_, _, err = setStyle.Call(handle, uintptr(index), style|wsExNoActivate|wsExToolWindow)
	if err != windows.ERROR_SUCCESS {
		return err
	}
	return nil
}
