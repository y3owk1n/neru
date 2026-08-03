package ports

import "context"

// EventTapPort is the interface for capturing keyboard events.
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
	// "__modifier_<name>_down/up" strings for sticky modifier toggling.
	SetStickyModifierToggle(enabled bool)

	// SetKeyboardLayout configures the reference keyboard layout used for key translation.
	// Returns false when an explicit layout ID is provided but cannot be resolved.
	SetKeyboardLayout(layoutID string) bool

	// PostModifierEvent simulates a physical modifier key press or release.
	// modifier must be one of "cmd", "shift", "alt", "ctrl".
	PostModifierEvent(modifier string, isDown bool)

	// Destroy cleans up the event tap resources.
	Destroy()
}

// OverlayKeyboardPassthroughReporter is an optional EventTapPort extension:
// whether an indicator overlay may drop exclusive keyboard capture so scroll
// reaches the focused app. Only the event tap can answer it — it needs a
// working uinput scroll device and no active evdev grab (a wlroots grab
// deactivates the focused toplevel). Implemented by the Linux Wayland evdev
// backend; treat a missing implementation as "keep capture".
type OverlayKeyboardPassthroughReporter interface {
	// AllowsOverlayKeyboardPassthrough reports whether an indicator overlay
	// can safely give up exclusive keyboard capture right now.
	AllowsOverlayKeyboardPassthrough() bool
}

// IPCPort is the interface for inter-process communication.
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
