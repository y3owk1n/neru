package app

import (
	"context"

	"github.com/y3owk1n/neru/internal/infra/hotkeys"
	"github.com/y3owk1n/neru/internal/infra/ipc"
	"go.uber.org/zap"
)

// hotkeyService defines the interface for hotkey management.
// It provides methods for registering and unregistering global hotkeys.
type hotkeyService interface {
	// Register registers a new hotkey with the given key string and callback.
	// Returns a HotkeyID that can be used to unregister the hotkey later.
	Register(keyString string, callback hotkeys.Callback) (hotkeys.HotkeyID, error)

	// UnregisterAll unregisters all registered hotkeys.
	UnregisterAll()
}

// eventTap defines the interface for event tap management.
// Event taps are used to intercept and handle keyboard events globally.
type eventTap interface {
	// Enable enables the event tap to start capturing events.
	Enable()

	// Disable disables the event tap to stop capturing events.
	Disable()

	// Destroy cleans up the event tap resources.
	Destroy()

	// SetHotkeys configures which hotkeys the event tap should monitor.
	SetHotkeys(hotkeys []string)
}

// ipcServer defines the interface for IPC server management.
// The IPC server handles communication between the daemon and CLI client.
type ipcServer interface {
	// Start starts the IPC server to begin accepting commands.
	Start()

	// Stop stops the IPC server and cleans up resources.
	Stop() error
}

// eventTapFactory defines the interface for creating event tap instances.
// This factory pattern enables dependency injection and testing.
type eventTapFactory interface {
	// New creates a new event tap with the given callback and logger.
	New(callback func(string), logger *zap.Logger) eventTap
}

// ipcServerFactory defines the interface for creating IPC server instances.
// This factory pattern enables dependency injection and testing.
type ipcServerFactory interface {
	// New creates a new IPC server with the given handler and logger.
	New(
		handler func(context.Context, ipc.Command) ipc.Response,
		logger *zap.Logger,
	) (ipcServer, error)
}

// deps holds optional dependencies for testing and dependency injection.
// When nil, default implementations are used.
type deps struct {
	// Hotkeys is an optional hotkey service implementation.
	Hotkeys hotkeyService

	// EventTapFactory is an optional event tap factory implementation.
	EventTapFactory eventTapFactory

	// IPCServerFactory is an optional IPC server factory implementation.
	IPCServerFactory ipcServerFactory
}
