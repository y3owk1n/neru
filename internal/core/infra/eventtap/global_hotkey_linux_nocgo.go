//go:build linux && !cgo

// internal/core/infra/eventtap/global_hotkey_linux_nocgo.go
// No-op GlobalHotkeyListener for builds without cgo (evdev needs cgo).
// Does nothing; exists only so the hotkey manager compiles without cgo.

package eventtap

import "go.uber.org/zap"

// GlobalHotkeyListener is a no-op stub when cgo is disabled.
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
