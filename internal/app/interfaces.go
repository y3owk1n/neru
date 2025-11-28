package app

import (
	"context"

	"github.com/y3owk1n/neru/internal/core/infra/appwatcher"
	"github.com/y3owk1n/neru/internal/core/infra/hotkeys"
	"github.com/y3owk1n/neru/internal/ui/overlay"
)

// HotkeyService defines the interface for hotkey management.
// It provides methods for registering and unregistering global hotkeys.
type HotkeyService interface {
	// Register registers a new hotkey with the given key string and callback.
	// Returns a HotkeyID that can be used to unregister the hotkey later.
	Register(keyString string, callback hotkeys.Callback) (hotkeys.HotkeyID, error)

	// UnregisterAll unregisters all registered hotkeys.
	UnregisterAll()
}

// OverlayManager defines the interface for overlay window management.
type OverlayManager = overlay.ManagerInterface

// Watcher defines the interface for application lifecycle monitoring.
type Watcher interface {
	Start()
	Stop()
	OnActivate(callback appwatcher.AppCallback)
	OnDeactivate(callback appwatcher.AppCallback)
	OnTerminate(callback appwatcher.AppCallback)
	OnScreenParametersChanged(callback func())
}

// ModeService defines the common interface for mode-specific services.
// This ensures grid, hints, and scroll services have identical APIs.
type ModeService interface {
	// Show activates the mode's overlay/interface.
	Show(ctx context.Context) error

	// Hide deactivates the mode's overlay/interface.
	Hide(ctx context.Context) error
}
