//go:build linux && !cgo

package linux

import (
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/derrors"
)

// GlobalHotkeyListener is the stub for builds without cgo, which evdev needs.
// It watches nothing and exists so the hotkey manager compiles.
type GlobalHotkeyListener struct{}

// NewGlobalHotkeyListener returns a stub listener.
func NewGlobalHotkeyListener(_ *zap.Logger) *GlobalHotkeyListener {
	return &GlobalHotkeyListener{}
}

// SetBinding is a no-op without cgo.
func (l *GlobalHotkeyListener) SetBinding(_ string, _, _ func()) {}

// ClearBindings is a no-op without cgo.
func (l *GlobalHotkeyListener) ClearBindings() {}

// Start reports CodeNotSupported without cgo. Answering nil would tell the
// hotkey manager the config keybindings are live while nothing watches.
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
