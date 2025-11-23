package app

import (
	"context"

	"github.com/y3owk1n/neru/internal/infra/appwatcher"
	"github.com/y3owk1n/neru/internal/infra/hotkeys"
	"github.com/y3owk1n/neru/internal/infra/ipc"
	"github.com/y3owk1n/neru/internal/ui/overlay"
	"go.uber.org/zap"
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

// EventTap defines the interface for event tap management.
// Event taps are used to intercept and handle keyboard events globally.
type EventTap interface {
	// Enable enables the event tap to start capturing events.
	Enable()

	// Disable disables the event tap to stop capturing events.
	Disable()

	// Destroy cleans up the event tap resources.
	Destroy()

	// SetHotkeys configures which hotkeys the event tap should monitor.
	SetHotkeys(hotkeys []string)
}

// IPCServer defines the interface for IPC server management.
// The IPC server handles communication between the daemon and CLI client.
type IPCServer interface {
	// Start starts the IPC server to begin accepting commands.
	Start()

	// Stop stops the IPC server and cleans up resources.
	Stop() error
}

// eventTapFactory defines the interface for creating event tap instances.
// This factory pattern enables dependency injection and testing.
type eventTapFactory interface {
	// New creates a new event tap with the given callback and logger.
	New(callback func(string), logger *zap.Logger) EventTap
}

// ipcServerFactory defines the interface for creating IPC server instances.
// This factory pattern enables dependency injection and testing.
type ipcServerFactory interface {
	// New creates a new IPC server with the given handler and logger.
	New(
		handler func(context.Context, ipc.Command) ipc.Response,
		logger *zap.Logger,
	) (IPCServer, error)
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

// overlayManagerFactory defines the interface for creating overlay managers.
type overlayManagerFactory interface {
	New(logger *zap.Logger) OverlayManager
}

// watcherFactory defines the interface for creating watcher instances.
// This factory pattern enables dependency injection and testing.
type watcherFactory interface {
	// New creates a new watcher with the given logger.
	New(logger *zap.Logger) Watcher
}

// deps holds optional dependencies for testing and dependency injection.
// When nil, default implementations are used.
type deps struct {
	// Hotkeys is an optional hotkey service implementation.
	Hotkeys HotkeyService

	// EventTapFactory is an optional event tap factory implementation.
	EventTapFactory eventTapFactory

	// IPCServerFactory is an optional IPC server factory implementation.
	IPCServerFactory ipcServerFactory

	// OverlayManagerFactory is an optional overlay manager factory implementation.
	OverlayManagerFactory overlayManagerFactory

	// WatcherFactory is an optional app watcher factory implementation.
	WatcherFactory watcherFactory
}
