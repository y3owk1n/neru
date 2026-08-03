package app

import (
	"context"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/ports"
)

// HotkeyService and HotkeyReleaseService are app-layer aliases for the hotkey
// contracts. The contracts themselves live in ports so platform backends can
// target them without importing the app package, and so the app layer no longer
// has to name infra types (hotkeys.Callback, hotkeys.HotkeyID) in its own
// signatures.
type (
	// HotkeyService registers global hotkeys.
	HotkeyService = ports.HotkeyPort
	// HotkeyReleaseService is the optional press/release extension, which
	// only the macOS backend implements. Reach it by type assertion and fall
	// back to HotkeyService.Register.
	HotkeyReleaseService = ports.HotkeyReleaseRegistrar
)

// OverlayManager is the interface for overlay window management.
type OverlayManager = overlay.ManagerInterface

// Watcher is the app-layer alias for the application lifecycle contract.
// The contract itself lives in ports so platform backends can target it
// without importing the app package.
type Watcher = ports.AppWatcherPort

// ModeService defines the common interface for mode-specific services.
// Grid, hints and scroll services all satisfy it, so their APIs stay aligned.
type ModeService interface {
	// Show activates the mode's overlay/interface.
	Show(ctx context.Context) error

	// Hide deactivates the mode's overlay/interface.
	Hide(ctx context.Context) error
}
