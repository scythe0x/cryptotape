//go:build !windows

package main

import "github.com/wailsapp/wails/v3/pkg/application"

func makeNonActivating(_ *application.WebviewWindow) error { return nil }
