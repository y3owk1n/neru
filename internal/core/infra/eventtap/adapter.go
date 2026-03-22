package eventtap

import (
	"context"
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/ports"
)

// Adapter implements ports.EventTapPort by wrapping the existing EventTap.
type Adapter struct {
	tap     *EventTap
	logger  *zap.Logger
	mu      sync.RWMutex
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
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.enabled
}

// SetHandler sets the function to call when a key is pressed.
func (a *Adapter) SetHandler(_ func(key string)) {
	a.logger.Warn("SetHandler called but EventTap doesn't support changing handler after creation")
}

// SetHotkeys configures which hotkeys the event tap should monitor.
// An empty slice is valid and clears all monitored hotkeys.
func (a *Adapter) SetHotkeys(hotkeys []string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(hotkeys) == 0 {
		a.logger.Debug("SetHotkeys called with empty slice — no hotkeys will be monitored")
	}

	a.tap.SetHotkeys(hotkeys)
}

// SetModifierPassthrough configures whether unbound modifier shortcuts should
// pass through to macOS and which shortcuts remain blacklisted.
func (a *Adapter) SetModifierPassthrough(enabled bool, blacklist []string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.tap.SetModifierPassthrough(enabled, blacklist)
}

// SetInterceptedModifierKeys configures modifier shortcuts the active mode
// still wants Neru to consume.
func (a *Adapter) SetInterceptedModifierKeys(keys []string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.tap.SetInterceptedModifierKeys(keys)
}

// SetPassthroughCallback registers a function to call when a modifier shortcut
// passes through to macOS.
func (a *Adapter) SetPassthroughCallback(cb func()) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.tap.SetPassthroughCallback(cb)
}

// SetStickyModifierToggle enables or disables sticky modifier toggle detection.
func (a *Adapter) SetStickyModifierToggle(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.tap.SetStickyModifierToggle(enabled)
}

// SetKeyboardLayout configures the reference keyboard layout used by key translation.
func (a *Adapter) SetKeyboardLayout(layoutID string) bool {
	return a.tap.SetKeyboardLayout(layoutID)
}

// PostModifierEvent simulates a physical modifier key press or release.
func (a *Adapter) PostModifierEvent(modifier string, isDown bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.tap.PostModifierEvent(modifier, isDown)
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
