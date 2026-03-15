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

	// SetHotkeys configures which hotkeys the event tap should monitor.
	SetHotkeys(hotkeys []string)

	// SetModifierPassthrough configures whether unbound Cmd/Ctrl/Alt shortcuts
	// should pass through to macOS, plus an optional blacklist of shortcuts to
	// keep consumed by Neru even when they are otherwise unbound.
	SetModifierPassthrough(enabled bool, blacklist []string)

	// SetInterceptedModifierKeys configures which modifier shortcuts the active
	// mode still wants Neru to consume while modifier passthrough is enabled.
	SetInterceptedModifierKeys(keys []string)

	// SetPassthroughCallback registers a function invoked when a modifier
	// shortcut passes through to macOS. Pass nil to clear.
	SetPassthroughCallback(cb func())

	// SetStickyModifierToggle enables or disables sticky modifier toggle detection.
	// When enabled, modifier key events are detected and callback is invoked with
	// "__modifier_<name>" strings for sticky modifier toggling.
	SetStickyModifierToggle(enabled bool)

	// SetKeyboardLayout configures the reference keyboard layout used for key translation.
	// Returns false when an explicit layout ID is provided but cannot be resolved.
	SetKeyboardLayout(layoutID string) bool

	// Destroy cleans up the event tap resources.
	Destroy()
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
	// Start starts the IPC server.
	Start(ctx context.Context) error

	// Stop stops the IPC server.
	Stop(ctx context.Context) error

	// Send sends a command to the IPC server.
	Send(ctx context.Context, command any) (any, error)

	// IsRunning returns true if the IPC server is running.
	IsRunning() bool
}
