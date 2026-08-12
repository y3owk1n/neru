package ports

import (
	"context"
	"image"
)

// ScreenCaptureConsent is the user's answer to the screen-recording permission
// prompt.
type ScreenCaptureConsent int

const (
	// ScreenCaptureGranted means recording is permitted and the caller may proceed.
	ScreenCaptureGranted ScreenCaptureConsent = iota
	// ScreenCaptureCanceled means the user declined; the caller should abandon
	// the operation but keep running.
	ScreenCaptureCanceled
	// ScreenCaptureQuit means the user asked to quit Neru entirely.
	ScreenCaptureQuit
)

// SystemPort is the platform system contract: health, capabilities, standard
// directories, process information, screen and cursor operations, permission
// gates, theme, and secure-input detection. Every consumer takes the whole
// port; optional per-platform extensions (RelativeCursorMover,
// CursorSynchronizer) are declared separately and reached by type assertion.
type SystemPort interface {
	// Health returns nil if the component is healthy, or an error if it is not.
	Health(ctx context.Context) error

	// Capabilities exposes runtime capability information.
	Capabilities() PlatformCapabilities

	// ConfigDir returns the platform-specific directory for configuration files.
	ConfigDir() (string, error)

	// UserDataDir returns the platform-specific directory for user data files.
	UserDataDir() (string, error)

	// LogDir returns the platform-specific directory for log files.
	LogDir() (string, error)

	// FocusedApplicationPID returns the PID of the currently focused application.
	FocusedApplicationPID(ctx context.Context) (int, error)

	// ApplicationNameByPID returns the name of the application with the given PID.
	ApplicationNameByPID(ctx context.Context, pid int) (string, error)

	// ApplicationBundleIDByPID returns the bundle ID (or equivalent) of the application with the given PID.
	ApplicationBundleIDByPID(ctx context.Context, pid int) (string, error)

	// ScreenBounds returns the bounds of the active screen.
	ScreenBounds(ctx context.Context) (image.Rectangle, error)

	// ScreenBoundsByName returns the bounds of the screen with the given
	// localized display name (case-insensitive). Returns the bounds and
	// true if found, or a zero rectangle and false if no screen matches.
	ScreenBoundsByName(ctx context.Context, name string) (image.Rectangle, bool, error)

	// ScreenNames returns the localized display names of all connected screens.
	// Returns nil or an empty slice when no screens are detected.
	ScreenNames(ctx context.Context) ([]string, error)

	// FocusedWindowBounds returns the bounds of the currently focused window.
	// Returns the bounds and true if a window was found, or a zero rectangle
	// and false if no focused window exists (e.g. the desktop is focused).
	//
	// A platform that cannot answer the question at all — a Wayland compositor
	// exposing no window geometry to other clients — returns CodeNotSupported
	// rather than false, so a caller scoping itself to the whole screen instead
	// knows it is falling back rather than obeying an answer.
	FocusedWindowBounds(ctx context.Context) (image.Rectangle, bool, error)

	// MoveCursorToPoint moves the mouse cursor to the specified point.
	// If bypassSmooth is true, smooth cursor configuration is bypassed.
	MoveCursorToPoint(ctx context.Context, point image.Point, bypassSmooth bool) error

	// WaitForCursorIdle blocks until any in-flight cursor movement has settled.
	// Implementations that do not animate cursor movement may return immediately.
	WaitForCursorIdle(ctx context.Context) error

	// CursorPosition returns the current cursor position.
	CursorPosition(ctx context.Context) (image.Point, error)

	// CheckPermissions verifies that accessibility permissions are granted.
	CheckPermissions(ctx context.Context) error

	// CheckScreenCapturePermission reports whether screen recording is
	// permitted, without prompting.
	//
	// Only the vision hint strategy needs this. Two backends gate screen
	// capture: macOS behind TCC, and KDE Plasma behind the xdg-desktop-portal
	// ScreenCast grant its compositor leaves as the only way to read the
	// screen. Everywhere else — X11, wlroots, Windows — there is no such gate
	// and the answer is true, which is a statement rather than a silent no-op.
	CheckScreenCapturePermission(ctx context.Context) bool

	// RequestScreenCapturePermission shows the platform's guidance for
	// granting screen recording and reports what the user chose.
	//
	// It blocks on a modal dialog, so callers must not hold a lock across it.
	// Platforms with no permission gate return ScreenCaptureGranted without
	// showing anything.
	//
	// It is also the only place a gated backend may put a consent prompt on
	// screen: a capture runs while the user waits for a mode to open, so the
	// prompt belongs here, where the caller has already taken it off its lock
	// and given it a budget sized for a human.
	//
	// Returning ScreenCaptureGranted implies a subsequent
	// CheckScreenCapturePermission reports true — callers retry a blocked
	// activation on Granted, and a consent that does not deliver the
	// permission would re-prompt forever.
	RequestScreenCapturePermission(ctx context.Context) ScreenCaptureConsent

	// IsDarkMode returns true if the platform's dark mode is currently active.
	IsDarkMode() bool

	// IsSecureInputEnabled returns true if secure input mode is currently active
	// (e.g. a password field is focused). On non-macOS platforms this always returns false.
	IsSecureInputEnabled() bool

	// ShowSecureInputNotification displays a platform notification informing the user
	// that mode activation was blocked because secure input is active.
	// On non-macOS platforms this is a no-op.
	ShowSecureInputNotification()

	// PlatformLabel returns a human-readable platform identifier that can be
	// used in startup notices and diagnostics. Unlike Capabilities().Platform,
	// this method performs no I/O or live probes — it returns a cached label
	// set at construction time.
	// Examples: "darwin", "windows", "linux/x11", "linux/wayland-kde".
	PlatformLabel() string

	// ShowAlert displays a native system alert/notification.
	// title   — brief summary shown as the alert heading (e.g. the error message)
	// message — detail text shown in the alert body (e.g. the config file path)
	ShowAlert(ctx context.Context, title, message string) error

	// ShowNotification displays a lightweight toast/banner notification.
	//
	// It carries an error because a notification is not always deliverable:
	// on Linux the session's notification daemon may be absent, which is an
	// ordinary state on a minimal compositor session, and a caller that
	// cannot tell "shown" from "dropped" has nothing to fall back to.
	// Platforms with no notification path at all report CodeNotSupported.
	//
	// Returning nil means the platform accepted the message, not that the
	// user has seen it — delivery can still be asynchronous.
	ShowNotification(ctx context.Context, title, message string) error
}

// RelativeCursorMover is an optional SystemPort extension: move the cursor by
// a delta natively, without a read-then-warp round trip. Optional extensions
// are declared here beside the port, opted into by implementing them, and
// reached by type assertion; the caller always needs a fallback. See
// docs/CROSS_PLATFORM.md ("The three tiers").
//
// Implemented by the Linux adapter, whose Wayland backends have no
// authoritative cursor query — warping to position+delta would compound the
// cache error, so the delta is applied directly. The darwin and Linux
// adapters also animate relative moves when smooth cursor is enabled (on
// wlroots the animation itself stays in delta space for the same
// cache-error reason). Fall back to CursorPosition + MoveCursorToPoint when
// unimplemented or handled == false.
type RelativeCursorMover interface {
	// MoveCursorBy moves the cursor by delta from its current position.
	// It returns handled == false when this particular backend cannot — or,
	// per configuration, chooses not to — apply the delta itself, which means
	// the caller should use its fallback.
	MoveCursorBy(ctx context.Context, delta image.Point) (handled bool, err error)
}

// CursorSettler is an optional SystemPort extension for platforms that
// animate cursor movement asynchronously. SettleCursor finishes any in-flight
// animation immediately — stopping it and placing the cursor at the endpoint
// it was animating toward — so a position-dependent action that fires
// mid-animation acts at the point the user aimed for.
//
// Callers resolving an action's target point from the cursor call this before
// reading the position; plain observers (indicator followers, pollers) read
// the position directly so animations are not cut short. Treat a missing
// implementation as "already settled".
type CursorSettler interface {
	// SettleCursor finishes any in-flight cursor animation immediately.
	SettleCursor(ctx context.Context) error
}

// CursorSynchronizer is an optional SystemPort extension for platforms whose
// cursor position is cached client-side and can drift from the compositor's.
//
// Implemented by the Linux adapter: on Wayland a client cannot query the
// pointer position directly, so the adapter keeps a cache that a plain
// user-driven mouse move invalidates. Modes call this before activation so the
// first cursor-follow lands correctly.
//
// Callers must treat a missing implementation as "already in sync" — never as
// an error.
type CursorSynchronizer interface {
	// SyncCursorPosition refreshes the adapter's cached cursor position from
	// the platform's authoritative source.
	SyncCursorPosition(ctx context.Context) error
}
