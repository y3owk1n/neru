// Package tap is the contract every event-tap backend implements.
//
// It exists as its own package for the same reason accessibility/ax does: the
// backends must name these types to satisfy them, and the adapter that selects
// a backend must import the backends. Both sides depend on this leaf instead of
// on each other.
package tap

// Callback receives each key the tap captured, already normalised to Neru's
// key vocabulary.
type Callback func(key string)

// PassthroughCallback is invoked when a key the tap was asked to pass through
// reaches the focused application.
type PassthroughCallback func()

// Tap is the platform keyboard capture backend: Quartz on macOS, evdev or X11
// on Linux, a low-level hook on Windows.
//
// The methods are what internal/adapter/eventtap.Adapter calls. Anything a
// platform cannot do is a no-op or returns false rather than an error — a tap
// that cannot set a keyboard layout should still capture keys.
type Tap interface {
	// Enable starts capturing. Calling it on an enabled tap is a no-op.
	Enable()
	// Disable stops capturing without tearing the tap down.
	Disable()
	// Destroy releases the tap and its OS resources.
	Destroy()

	// SetHotkeys sets the keys the tap captures rather than passes through.
	SetHotkeys(hotkeys []string)
	// SetModifierPassthrough lets modifiers reach the focused app, except for
	// the blacklisted ones.
	SetModifierPassthrough(enabled bool, blacklist []string)
	// SetInterceptedModifierKeys names the modifier keys the tap swallows.
	SetInterceptedModifierKeys(keys []string)
	// SetStickyModifierToggle enables the sticky-modifier behavior.
	SetStickyModifierToggle(enabled bool)
	// SetPassthroughCallback is invoked when a passthrough key is seen.
	SetPassthroughCallback(cb PassthroughCallback)

	// SetKeyboardLayout switches the layout used to translate keycodes, and
	// reports whether the backend could honor it.
	SetKeyboardLayout(layoutID string) bool
	// PostModifierEvent synthesizes a modifier press or release.
	PostModifierEvent(modifier string, isDown bool)
}
