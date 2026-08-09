//go:build linux && !cgo

package linux

import (
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/derrors"
)

// GlobalHotkeyListener is the stub for builds without cgo, which evdev needs.
// It watches nothing; it exists so the hotkey manager compiles, and Start says
// so rather than letting the manager believe a reader is running.
type GlobalHotkeyListener struct{}

// NewGlobalHotkeyListener returns a stub listener.
func NewGlobalHotkeyListener(_ *zap.Logger) *GlobalHotkeyListener {
	return &GlobalHotkeyListener{}
}

// SetBinding is a no-op without cgo.
func (l *GlobalHotkeyListener) SetBinding(_ string, _ func()) {}

// ClearBindings is a no-op without cgo.
func (l *GlobalHotkeyListener) ClearBindings() {}

// Start reports CodeNotSupported without cgo: evdev needs it, so there is no
// reader to start. Answering nil would tell the hotkey manager the user's
// config keybindings are live while nothing is watching the keyboard.
func (l *GlobalHotkeyListener) Start() error {
	return derrors.New(
		derrors.CodeNotSupported,
		"Wayland global hotkeys require CGO-enabled Linux builds",
	)
}

// Stop is a no-op without cgo.
func (l *GlobalHotkeyListener) Stop() {}

// StopWithTimeout is a no-op stub without cgo.
func (l *GlobalHotkeyListener) StopWithTimeout(_ time.Duration) bool { return true }

// IsRunning returns false without cgo (evdev is unavailable).
func (l *GlobalHotkeyListener) IsRunning() bool { return false }

// DeviceCount returns 0 without cgo.
func (l *GlobalHotkeyListener) DeviceCount() int { return 0 }
