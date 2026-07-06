//go:build linux

package app

import (
	"go.uber.org/zap"

	infrasystray "github.com/y3owk1n/neru/internal/core/infra/systray"
)

// initializePlatformLogger is a no-op on Linux.
func initializePlatformLogger(_ *zap.Logger) {}

// platformQuit unblocks the systray loop so App.Quit() (tray menu or signal)
// actually stops the Linux daemon; without it systray.Run/RunHeadless keeps
// the process alive after a clean quit. Mirrors darwin/windows.
func platformQuit() {
	infrasystray.Quit()
}
