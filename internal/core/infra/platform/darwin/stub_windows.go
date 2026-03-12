//go:build windows

package darwin

import (
	"context"
	"image"
	"unsafe"

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

// IsSecureInputEnabled is a stub.
func (s *SystemAdapter) IsSecureInputEnabled() bool { return false }

// ShowSecureInputNotification is a stub.
func (s *SystemAdapter) ShowSecureInputNotification() {}

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
func StartAppWatcher() {}

// StopAppWatcher is a stub.
func StopAppWatcher() {}

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

// CursorPosition is a stub.
func CursorPosition() image.Point { return image.Point{} }

// HasClickAction is a stub.
func HasClickAction(element unsafe.Pointer) bool { return false }

// IsDarkMode is a stub (package-level).
func IsDarkMode() bool { return false }

// FreeCString is a stub.
func FreeCString(ptr unsafe.Pointer) {}

// MallocCallbackContext is a stub.
func MallocCallbackContext(callbackID, generation uint64) unsafe.Pointer { return nil }

// FreeCallbackContext is a stub.
func FreeCallbackContext(ptr unsafe.Pointer) {}

// SetReferenceKeyboardLayout is a stub.
func SetReferenceKeyboardLayout(inputSourceID string) bool { return true }

// RegisterHotkey is a stub.
func RegisterHotkey(
	keyCode, modifiers, hotkeyID int,
	callback unsafe.Pointer,
	userData unsafe.Pointer,
) bool {
	return false
}

// UnregisterHotkey is a stub.
func UnregisterHotkey(hotkeyID int) {}

// UnregisterAllHotkeys is a stub.
func UnregisterAllHotkeys() {}

// ParseKeyString is a stub.
func ParseKeyString(keyString string) (int, int, bool) { return 0, 0, false }

// HotkeyHandler defines the signature for hotkey event handlers (stub).
type HotkeyHandler func(hotkeyID int)

// SetHotkeyHandler is a stub.
func SetHotkeyHandler(handler HotkeyHandler) {}

// GetHotkeyCallbackBridge is a stub.
func GetHotkeyCallbackBridge() unsafe.Pointer { return nil }

// Logger returns nil on non-darwin (stub).
func Logger() *zap.Logger { return nil }
