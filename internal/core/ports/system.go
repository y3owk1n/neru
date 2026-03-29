package ports

import (
	"context"
	"image"
)

// ScreenManagement defines the interface for screen and cursor operations.
type ScreenManagement interface {
	// ScreenBounds returns the bounds of the active screen.
	ScreenBounds(ctx context.Context) (image.Rectangle, error)

	// ScreenBoundsByName returns the bounds of the screen with the given
	// localized display name (case-insensitive). Returns the bounds and
	// true if found, or a zero rectangle and false if no screen matches.
	ScreenBoundsByName(ctx context.Context, name string) (image.Rectangle, bool, error)

	// ScreenNames returns the localized display names of all connected screens.
	// Returns nil or an empty slice when no screens are detected.
	ScreenNames(ctx context.Context) ([]string, error)

	// MoveCursorToPoint moves the mouse cursor to the specified point.
	// If bypassSmooth is true, smooth cursor configuration is bypassed.
	MoveCursorToPoint(ctx context.Context, point image.Point, bypassSmooth bool) error

	// WaitForCursorIdle blocks until any in-flight cursor movement has settled.
	// Implementations that do not animate cursor movement may return immediately.
	WaitForCursorIdle(ctx context.Context) error

	// CursorPosition returns the current cursor position.
	CursorPosition(ctx context.Context) (image.Point, error)
}

// PermissionManagement defines the interface for accessibility permissions.
type PermissionManagement interface {
	// CheckPermissions verifies that accessibility permissions are granted.
	CheckPermissions(ctx context.Context) error
}

// FileSystemPort defines the interface for platform-specific file system operations.
type FileSystemPort interface {
	// ConfigDir returns the platform-specific directory for configuration files.
	ConfigDir() (string, error)

	// UserDataDir returns the platform-specific directory for user data files.
	UserDataDir() (string, error)

	// LogDir returns the platform-specific directory for log files.
	LogDir() (string, error)
}

// ProcessPort defines the interface for platform-specific process management.
type ProcessPort interface {
	// FocusedApplicationPID returns the PID of the currently focused application.
	FocusedApplicationPID(ctx context.Context) (int, error)

	// ApplicationNameByPID returns the name of the application with the given PID.
	ApplicationNameByPID(ctx context.Context, pid int) (string, error)

	// ApplicationBundleIDByPID returns the bundle ID (or equivalent) of the application with the given PID.
	ApplicationBundleIDByPID(ctx context.Context, pid int) (string, error)
}

// ThemeProviderPort defines the interface for platform-specific theme information.
type ThemeProviderPort interface {
	// IsDarkMode returns true if the platform's dark mode is currently active.
	IsDarkMode() bool
}

// SecureInputPort defines the interface for secure input detection and notification.
type SecureInputPort interface {
	// IsSecureInputEnabled returns true if secure input mode is currently active
	// (e.g. a password field is focused). On non-macOS platforms this always returns false.
	IsSecureInputEnabled() bool

	// ShowSecureInputNotification displays a platform notification informing the user
	// that mode activation was blocked because secure input is active.
	// On non-macOS platforms this is a no-op.
	ShowSecureInputNotification()
}

// SystemPort combines various platform-specific system interfaces.
type SystemPort interface {
	HealthCheck
	CapabilityReporter
	FileSystemPort
	ProcessPort
	ScreenManagement
	PermissionManagement
	ThemeProviderPort
	SecureInputPort

	// ShowAlert displays a native system alert/notification.
	// title   — brief summary shown as the alert heading (e.g. the error message)
	// message — detail text shown in the alert body (e.g. the config file path)
	ShowAlert(ctx context.Context, title, message string) error

	// ShowNotification displays a lightweight toast/banner notification.
	ShowNotification(title, message string)
}
