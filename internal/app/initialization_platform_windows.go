//go:build windows

package app

import (
	"go.uber.org/zap"

	infrasystray "github.com/y3owk1n/neru/internal/core/infra/systray"
)

// initializePlatformLogger is a no-op on Windows.
func initializePlatformLogger(_ *zap.Logger) {}

// platformQuit posts WM_QUIT to unblock the Windows message pump.
func platformQuit() {
	infrasystray.Quit()
}
