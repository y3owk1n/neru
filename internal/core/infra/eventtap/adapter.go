package eventtap

import (
	"context"
	"sync"

	"github.com/y3owk1n/neru/internal/core/ports"
	"go.uber.org/zap"
)

// Adapter implements ports.EventTapPort by wrapping the existing EventTap.
type Adapter struct {
	tap     *EventTap
	logger  *zap.Logger
	mu      sync.Mutex
	enabled bool
}

// NewAdapter creates a new event tap adapter.
func NewAdapter(tap *EventTap, logger *zap.Logger) *Adapter {
	return &Adapter{
		tap:    tap,
		logger: logger,
	}
}

// Enable enables the event tap.
func (a *Adapter) Enable(_ context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.tap.Enable()
	a.enabled = true

	return nil
}

// Disable disables the event tap.
func (a *Adapter) Disable(_ context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.tap.Disable()
	a.enabled = false

	return nil
}

// IsEnabled returns true if event capture is active.
func (a *Adapter) IsEnabled() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.enabled
}

// SetHandler sets the function to call when a key is pressed.
func (a *Adapter) SetHandler(_ func(key string)) {
	a.logger.Warn("SetHandler called but EventTap doesn't support changing handler after creation")
}

// SetHotkeys configures which hotkeys the event tap should monitor.
func (a *Adapter) SetHotkeys(hotkeys []string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(hotkeys) == 0 {
		a.logger.Warn("SetHotkeys called with empty hotkeys slice")

		return
	}

	a.tap.SetHotkeys(hotkeys)
}

// Destroy cleans up the event tap resources.
func (a *Adapter) Destroy() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.tap.Destroy()
	a.enabled = false
}

// Ensure Adapter implements ports.EventTapPort.
var _ ports.EventTapPort = (*Adapter)(nil)
