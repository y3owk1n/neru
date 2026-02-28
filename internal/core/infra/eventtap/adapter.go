package eventtap

import (
	"context"
	"sync/atomic"

	"github.com/y3owk1n/neru/internal/core/ports"
	"go.uber.org/zap"
)

// Adapter implements ports.EventTapPort by wrapping the existing EventTap.
type Adapter struct {
	tap     *EventTap
	logger  *zap.Logger
	enabled atomic.Bool
}

// NewAdapter creates a new event tap adapter.
func NewAdapter(tap *EventTap, logger *zap.Logger) *Adapter {
	adapter := &Adapter{
		tap:    tap,
		logger: logger,
	}
	adapter.enabled.Store(false)

	return adapter
}

// Enable enables the event tap.
func (a *Adapter) Enable(_ context.Context) error {
	a.tap.Enable()
	a.enabled.Store(true)

	return nil
}

// Disable disables the event tap.
func (a *Adapter) Disable(_ context.Context) error {
	a.tap.Disable()
	a.enabled.Store(false)

	return nil
}

// IsEnabled returns true if event capture is active.
func (a *Adapter) IsEnabled() bool {
	return a.enabled.Load()
}

// SetHandler sets the function to call when a key is pressed.
func (a *Adapter) SetHandler(_ func(key string)) {
	a.logger.Warn("SetHandler called but EventTap doesn't support changing handler after creation")
}

// SetHotkeys configures which hotkeys the event tap should monitor.
func (a *Adapter) SetHotkeys(hotkeys []string) {
	if len(hotkeys) == 0 {
		a.logger.Warn("SetHotkeys called with empty hotkeys slice")

		return
	}

	a.tap.SetHotkeys(hotkeys)
}

// Destroy cleans up the event tap resources.
func (a *Adapter) Destroy() {
	a.tap.Destroy()
	a.enabled.Store(false)
}

// Ensure Adapter implements ports.EventTapPort.
var _ ports.EventTapPort = (*Adapter)(nil)
