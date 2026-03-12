//go:build windows

package darwin

import (
	"context"
	"image"

	"go.uber.org/zap"
)

// SystemAdapter is a stub implementation of ports.SystemPort for Windows.
type SystemAdapter struct{}

// NewSystemAdapter returns a new SystemAdapter stub.
func NewSystemAdapter() *SystemAdapter {
	return &SystemAdapter{}
}

// Health is a stub.
func (s *SystemAdapter) Health(ctx context.Context) error { return nil }

// ConfigDir is a stub.
func (s *SystemAdapter) ConfigDir() (string, error) { return "", nil }

// UserDataDir is a stub.
func (s *SystemAdapter) UserDataDir() (string, error) { return "", nil }

// LogDir is a stub.
func (s *SystemAdapter) LogDir() (string, error) { return "", nil }

// FocusedApplicationPID is a stub.
func (s *SystemAdapter) FocusedApplicationPID(ctx context.Context) (int, error) { return 0, nil }

// ApplicationNameByPID is a stub.
func (s *SystemAdapter) ApplicationNameByPID(ctx context.Context, pid int) (string, error) {
	return "", nil
}

// ApplicationBundleIDByPID is a stub.
func (s *SystemAdapter) ApplicationBundleIDByPID(ctx context.Context, pid int) (string, error) {
	return "", nil
}

// ScreenBounds is a stub.
func (s *SystemAdapter) ScreenBounds(ctx context.Context) (image.Rectangle, error) {
	return image.Rectangle{}, nil
}

// MoveCursorToPoint is a stub.
func (s *SystemAdapter) MoveCursorToPoint(
	ctx context.Context,
	point image.Point,
	bypassSmooth bool,
) error {
	return nil
}

// CursorPosition is a stub.
func (s *SystemAdapter) CursorPosition(ctx context.Context) (image.Point, error) {
	return image.Point{}, nil
}

// CheckPermissions is a stub.
func (s *SystemAdapter) CheckPermissions(ctx context.Context) error { return nil }

// IsDarkMode is a stub.
func (s *SystemAdapter) IsDarkMode() bool { return false }

// ShowAlert is a stub.
func (s *SystemAdapter) ShowAlert(ctx context.Context, title, message string) error { return nil }

// ShowConfigValidationError is a stub.
func ShowConfigValidationError(errorMessage, configPath string) {}

// ActiveScreenBounds returns the active screen bounds on Windows (stub).
func ActiveScreenBounds() image.Rectangle {
	return image.Rectangle{}
}

// AppWatcherInterface interface defines callbacks for application lifecycle events.
type AppWatcherInterface interface {
	HandleLaunch(appName, bundleID string)
	HandleTerminate(appName, bundleID string)
	HandleActivate(appName, bundleID string)
	HandleDeactivate(appName, bundleID string)
	HandleScreenParametersChanged()
}

// SetAppWatcher is a stub.
func SetAppWatcher(watcher AppWatcherInterface) {}

// StartAppWatcher is a stub.
func StartAppWatcher() error { return nil }

// StopAppWatcher is a stub.
func StopAppWatcher() error { return nil }

// ShowNotification is a stub.
func ShowNotification(title, message string) {}

// SetApplicationAttribute is a stub.
func SetApplicationAttribute(pid int, attribute string, value bool) bool { return false }

// IsSecureInputEnabled is a stub.
func IsSecureInputEnabled() bool { return false }

// ShowSecureInputNotification is a stub.
func ShowSecureInputNotification() {}

// InitializeLogger initializes the logger for the macOS platform package (Windows stub).
func InitializeLogger(logger *zap.Logger) {}

// SetThemeChangeHandler sets the callback function to be called when the system theme changes (Windows stub).
func SetThemeChangeHandler(handler func(bool)) {}

// StartThemeObserver starts the system theme observer (Windows stub).
func StartThemeObserver() {}

// StopThemeObserver stops the system theme observer (Windows stub).
func StopThemeObserver() {}

// Mouse functions (Windows stub)
func SetLeftMouseDown(down bool, pos image.Point)                         {}
func IsLeftMouseDown() bool                                               { return false }
func GetLastMouseDownPosition() image.Point                               { return image.Point{} }
func ClearLeftMouseDownState()                                            {}
func EnsureMouseUp()                                                      {}
func MoveMouse(point image.Point, bypassSmooth bool)                      {}
func MoveMouseSmooth(end image.Point, steps, delay int, eventType uint32) {}
func LeftClickAtPoint(point image.Point, restoreCursor bool) error        { return nil }
func RightClickAtPoint(point image.Point, restoreCursor bool) error       { return nil }
func MiddleClickAtPoint(point image.Point, restoreCursor bool) error      { return nil }
func LeftMouseDownAtPoint(point image.Point) error                        { return nil }
func LeftMouseUpAtPoint(point image.Point) error                          { return nil }
func LeftMouseUp() error                                                  { return nil }
func ScrollAtCursor(deltaX, deltaY int) error                             { return nil }
