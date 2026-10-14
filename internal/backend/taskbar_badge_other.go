//go:build !windows

package backend

import "github.com/wailsapp/wails/v3/pkg/application"

// setTaskbarBadge has no counterpart outside Windows.
func setTaskbarBadge(_ *application.WebviewWindow, _ int) {}
