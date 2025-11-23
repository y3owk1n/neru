package ports

import "context"

// EventTapPort defines the interface for capturing keyboard events.
// Implementations handle platform-specific event monitoring.
type EventTapPort interface {
	// Enable starts capturing keyboard events.
	Enable(ctx context.Context) error

	// Disable stops capturing keyboard events.
	Disable(ctx context.Context) error

	// IsEnabled returns true if event capture is active.
	IsEnabled() bool

	// SetHandler sets the function to call when a key is pressed.
	SetHandler(handler func(key string))
}

// HotkeyPort defines the interface for global hotkey registration.
// Implementations handle platform-specific hotkey APIs.
type HotkeyPort interface {
	// Register registers a hotkey with the given callback.
	Register(ctx context.Context, hotkey string, callback func() error) error

	// Unregister removes a previously registered hotkey.
	Unregister(ctx context.Context, hotkey string) error

	// UnregisterAll removes all registered hotkeys.
	UnregisterAll(ctx context.Context) error

	// IsRegistered returns true if the hotkey is currently registered.
	IsRegistered(hotkey string) bool
}

// IPCPort defines the interface for inter-process communication.
// Implementations handle the IPC server and client functionality.
type IPCPort interface {
	// Serve starts the IPC server and blocks until context is canceled.
	Serve(ctx context.Context) error

	// Send sends a command to the IPC server.
	Send(ctx context.Context, command any) (any, error)

	// IsRunning returns true if the IPC server is running.
	IsRunning() bool
}
