//go:build windows

package eventtap

import (
	"context"

	"go.uber.org/zap"
)

// Callback defines the function signature for handling key press events.
type Callback func(key string)

// PassthroughCallback is invoked when a modifier shortcut passes through to the system.
type PassthroughCallback func()

// EventTap represents a keyboard event interceptor (Windows stub).
type EventTap struct {
	logger *zap.Logger
}

// NewEventTap initializes a new event tap (Windows stub).
func NewEventTap(_ Callback, logger *zap.Logger) *EventTap {
	return &EventTap{logger: logger}
}

// Enable enables the event tap (Windows stub).
func (et *EventTap) Enable() {}

// Disable disables the event tap (Windows stub).
func (et *EventTap) Disable() {}

// Destroy destroys the event tap (Windows stub).
func (et *EventTap) Destroy() {}

// SetHotkeys sets the hotkeys (Windows stub).
func (et *EventTap) SetHotkeys(_ []string) {}

// SetModifierPassthrough sets modifier passthrough (Windows stub).
func (et *EventTap) SetModifierPassthrough(_ bool, _ []string) {}

// SetInterceptedModifierKeys sets intercepted modifier keys (Windows stub).
func (et *EventTap) SetInterceptedModifierKeys(_ []string) {}

// SetPassthroughCallback sets the passthrough callback (Windows stub).
func (et *EventTap) SetPassthroughCallback(_ PassthroughCallback) {}

// SetStickyModifierToggle enables or disables sticky modifier toggle detection (Windows stub).
func (et *EventTap) SetStickyModifierToggle(_ bool) {}

// PostModifierEvent simulates a physical modifier key press or release (Windows stub).
func (et *EventTap) PostModifierEvent(_ string, _ bool) {}

// SetKeyboardLayout sets the keyboard layout (Windows stub).
func (et *EventTap) SetKeyboardLayout(_ string) bool { return true }

// IsEnabled returns whether the event tap is enabled (Windows stub).
func (et *EventTap) IsEnabled() bool { return false }

// SetHandler sets the key handler (Windows stub).
func (et *EventTap) SetHandler(_ func(key string)) {}

// EnableWithContext enables the event tap with context (Windows stub).
func (et *EventTap) EnableWithContext(_ context.Context) error { return nil }

// DisableWithContext disables the event tap with context (Windows stub).
func (et *EventTap) DisableWithContext(_ context.Context) error { return nil }
