//go:build windows

package app

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/eventtap/tap"
	infrasystray "github.com/y3owk1n/neru/internal/adapter/systray"
)

// initializePlatformLogger is a no-op on Windows.
func initializePlatformLogger(_ *zap.Logger) {}

// registerSyntheticModifierSink has nothing to wire on Windows. The injection
// backend does press real modifier keys, but SendInput carries a tag on each
// one (neruInjectedTag, internal/adapter/platform/windows/input.go) that the
// keyboard hook reads off the event itself — so the event says whose it is, and
// nothing has to be told in advance.
func registerSyntheticModifierSink(_ tap.Tap, _ *zap.Logger) {}

// platformQuit posts WM_QUIT to unblock the Windows message pump.
func platformQuit() {
	infrasystray.Quit()
}
