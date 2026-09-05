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

// HotkeyReleaseRegistrar is an optional HotkeyPort extension for backends that
// can distinguish key press from key release.
//
// The rules for optional extensions are in system.go and apply here too.
//
// Implemented by all three managers, each from a different source: the macOS
// per-hotkey CGEventTaps see both edges; on Linux the evdev proxy reads every
// release and an XGrabKey delivers KeyRelease to the grabbing client; Win32's
// RegisterHotKey reports the press only, so the Windows manager polls the
// key's state while it is held. A hold is one press and one release however
// long it lasts — autorepeat fires nothing — which is what held-key repeat
// ([held_repeat]) is built on.
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
