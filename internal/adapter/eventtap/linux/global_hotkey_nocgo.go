//go:build linux && !cgo

package linux

import (
	"time"

	"go.uber.org/zap"
)

// GlobalHotkeyListener is a no-op stub when cgo is disabled.
//
// No-op GlobalHotkeyListener for builds without cgo (evdev needs cgo).
// Does nothing; exists only so the hotkey manager compiles without cgo.
type GlobalHotkeyListener struct{}

// NewGlobalHotkeyListener returns a stub listener.
func NewGlobalHotkeyListener(_ *zap.Logger) *GlobalHotkeyListener {
	return &GlobalHotkeyListener{}
}

// SetBinding is a no-op without cgo.
func (l *GlobalHotkeyListener) SetBinding(_ string, _ func()) {}

// ClearBindings is a no-op without cgo.
func (l *GlobalHotkeyListener) ClearBindings() {}

// Start is a no-op without cgo.
func (l *GlobalHotkeyListener) Start() error { return nil }

// Stop is a no-op without cgo.
func (l *GlobalHotkeyListener) Stop() {}

// StopWithTimeout is a no-op stub without cgo.
func (l *GlobalHotkeyListener) StopWithTimeout(_ time.Duration) bool { return true }

// IsRunning returns false without cgo (evdev is unavailable).
func (l *GlobalHotkeyListener) IsRunning() bool { return false }

// DeviceCount returns 0 without cgo.
func (l *GlobalHotkeyListener) DeviceCount() int { return 0 }
