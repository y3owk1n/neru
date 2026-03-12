package ports

import (
	"context"
	"image"
)

// ScreenManagement defines the interface for screen and cursor operations.
type ScreenManagement interface {
	// ScreenBounds returns the bounds of the active screen.
	ScreenBounds(ctx context.Context) (image.Rectangle, error)

	// MoveCursorToPoint moves the mouse cursor to the specified point.
	// If bypassSmooth is true, smooth cursor configuration is bypassed.
	MoveCursorToPoint(ctx context.Context, point image.Point, bypassSmooth bool) error

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
	FileSystemPort
	ProcessPort
	ScreenManagement
	PermissionManagement
	ThemeProviderPort
	SecureInputPort

	// ShowAlert displays a native system alert/notification.
	ShowAlert(ctx context.Context, title, message string) error
}
