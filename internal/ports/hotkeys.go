package ports

// HotkeyID identifies one registered hotkey for later unregistration.
type HotkeyID int

// HotkeyCallback is invoked when a registered hotkey fires.
//
// It runs on the platform's hotkey delivery goroutine, never on the event-tap
// thread. Implementations must not block in it.
type HotkeyCallback func()

// HotkeyPort registers global hotkeys — the shortcuts that reach Neru even when
// another application has focus.
//
// Unlike most ports these methods take no context: registration is a
// synchronous native call (CGEventTap creation, XGrabKey, RegisterHotKey) with
// no cancellation point, and a context parameter that could never be honored
// would be a lie in the signature.
//
// Implementations must report derrors.CodeHotkeyRegisterFailed when the
// platform refuses a binding, and the global_hotkeys entry in
// PlatformCapabilities must describe the mechanism and its caveats.
type HotkeyPort interface {
	// Register binds keyString and returns an ID for later unregistration.
	// keyString is in the canonical form produced by
	// config.CanonicalHotkeyForPlatform (e.g. "Ctrl+Shift+Space").
	Register(keyString string, callback HotkeyCallback) (HotkeyID, error)

	// Unregister releases a single hotkey. Unknown IDs are ignored.
	Unregister(hotkeyID HotkeyID)

	// UnregisterAll releases every hotkey this manager holds.
	UnregisterAll()
}

// Optional HotkeyPort extensions.
//
// See the equivalent block in system.go for the rules; the same three apply
// here — declare the interface in this package, give the caller a fallback, and
// document which backends implement it.

// HotkeyReleaseRegistrar is an optional HotkeyPort extension for backends that
// can distinguish key press from key release.
//
// Implemented by the macOS manager, whose per-hotkey CGEventTaps see both
// edges. X11's XGrabKey and Win32's RegisterHotKey deliver press only, so those
// backends do not implement it.
//
// Callers must fall back to Register — a press-only binding is a working
// binding; only hold-to-activate behavior is lost.
type HotkeyReleaseRegistrar interface {
	// RegisterWithRelease binds keyString with separate press and release
	// callbacks.
	RegisterWithRelease(
		keyString string,
		pressCallback HotkeyCallback,
		releaseCallback HotkeyCallback,
	) (HotkeyID, error)
}

// HotkeyHealthReporter is an optional HotkeyPort extension for backends whose
// hotkey grabs can be invalidated out from under them.
//
// Implemented by the Linux manager: an X11 server restart or a Wayland
// compositor reload drops existing grabs silently, so the sleep/resume handler
// probes health and re-registers rather than leaving the user with dead keys.
// macOS and Windows keep their registrations across sleep, so they do not
// implement it.
//
// Callers must treat a missing implementation as healthy.
type HotkeyHealthReporter interface {
	// HealthCheck reports whether the backend's hotkey grabs are still live.
	HealthCheck() bool
}
