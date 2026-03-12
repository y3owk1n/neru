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
func NewEventTap(callback Callback, logger *zap.Logger) *EventTap {
	return &EventTap{logger: logger}
}

func (et *EventTap) Enable()                                                 {}
func (et *EventTap) Disable()                                                {}
func (et *EventTap) Destroy()                                                {}
func (et *EventTap) SetHotkeys(hotkeys []string)                             {}
func (et *EventTap) SetModifierPassthrough(enabled bool, blacklist []string) {}
func (et *EventTap) SetInterceptedModifierKeys(keys []string)                {}
func (et *EventTap) SetPassthroughCallback(callback PassthroughCallback)     {}
func (et *EventTap) SetKeyboardLayout(layoutID string) bool                  { return true }
func (et *EventTap) IsEnabled() bool                                         { return false }
func (et *EventTap) SetHandler(handler func(key string))                     {}
func (et *EventTap) EnableWithContext(ctx context.Context) error             { return nil }
func (et *EventTap) DisableWithContext(ctx context.Context) error            { return nil }
