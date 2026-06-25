//go:build windows

// Package windows provides Windows-specific implementations of infrastructure components.
//
// Most methods currently return CodeNotSupported because Windows support is a
// work-in-progress. Contributors should replace each stub with a real
// implementation and remove the CodeNotSupported return when done.
// See docs/ARCHITECTURE.md for the contribution guide.
//
//nolint:godox // TODO comments are intentional contributor guidance for unimplemented stubs.
package windows

import (
	"context"
	"image"
	"os"
	"path/filepath"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// SystemAdapter implements ports.SystemPort for Windows.
type SystemAdapter struct{}

// NewSystemAdapter creates a new SystemAdapter.
func NewSystemAdapter() *SystemAdapter {
	return &SystemAdapter{}
}

// Health checks the health of the Windows system adapter.
func (s *SystemAdapter) Health(ctx context.Context) error {
	return nil
}

// Capabilities returns the current Windows capability surface.
func (s *SystemAdapter) Capabilities() ports.PlatformCapabilities {
	return ports.WindowsCapabilities()
}

// ConfigDir returns the Windows-specific configuration directory.
func (s *SystemAdapter) ConfigDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		appData = filepath.Join(home, "AppData", "Roaming")
	}

	return filepath.Join(appData, "neru"), nil
}

// UserDataDir returns the Windows-specific user data directory.
func (s *SystemAdapter) UserDataDir() (string, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		localAppData = filepath.Join(home, "AppData", "Local")
	}

	return filepath.Join(localAppData, "neru"), nil
}

// LogDir returns the Windows-specific log directory.
func (s *SystemAdapter) LogDir() (string, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		localAppData = filepath.Join(home, "AppData", "Local")
	}

	return filepath.Join(localAppData, "neru", "log"), nil
}

// FocusedApplicationIdentity returns the foreground app executable path and PID.
func FocusedApplicationIdentity() (string, int, error) {
	pid, err := focusedApplicationPID()
	if err != nil {
		return "", 0, err
	}

	bundleID, err := applicationBundleIDByPID(pid)
	if err != nil {
		return "", pid, err
	}

	return bundleID, pid, nil
}

// FocusedApplicationPID returns the PID of the currently focused application on Windows.
func (s *SystemAdapter) FocusedApplicationPID(ctx context.Context) (int, error) {
	err := ctx.Err()
	if err != nil {
		return 0, err
	}

	return focusedApplicationPID()
}

// ApplicationNameByPID returns the name of the application with the given PID on Windows.
func (s *SystemAdapter) ApplicationNameByPID(ctx context.Context, pid int) (string, error) {
	err := ctx.Err()
	if err != nil {
		return "", err
	}

	return applicationNameByPID(pid)
}

// ApplicationBundleIDByPID returns the executable path for the given PID on Windows.
func (s *SystemAdapter) ApplicationBundleIDByPID(ctx context.Context, pid int) (string, error) {
	err := ctx.Err()
	if err != nil {
		return "", err
	}

	return applicationBundleIDByPID(pid)
}

// ScreenBounds returns the bounds of the active screen on Windows.
func (s *SystemAdapter) ScreenBounds(ctx context.Context) (image.Rectangle, error) {
	err := ctx.Err()
	if err != nil {
		return image.Rectangle{}, err
	}

	return activeScreenBounds()
}

// ScreenBoundsByName returns the bounds of the screen with the given name on Windows.
func (s *SystemAdapter) ScreenBoundsByName(
	ctx context.Context,
	name string,
) (image.Rectangle, bool, error) {
	err := ctx.Err()
	if err != nil {
		return image.Rectangle{}, false, err
	}

	return screenBoundsByName(name)
}

// ScreenNames returns the display names of all connected screens on Windows.
func (s *SystemAdapter) ScreenNames(ctx context.Context) ([]string, error) {
	err := ctx.Err()
	if err != nil {
		return nil, err
	}

	return screenNames()
}

// FocusedWindowBounds returns the bounds of the currently focused window on Windows.
func (s *SystemAdapter) FocusedWindowBounds(
	ctx context.Context,
) (image.Rectangle, bool, error) {
	err := ctx.Err()
	if err != nil {
		return image.Rectangle{}, false, err
	}

	return focusedWindowBounds()
}

// MoveCursorToPoint moves the mouse cursor to the specified point on Windows.
func (s *SystemAdapter) MoveCursorToPoint(
	ctx context.Context,
	point image.Point,
	_ bool,
) error {
	err := ctx.Err()
	if err != nil {
		return err
	}

	return moveCursorTo(point)
}

// WaitForCursorIdle returns immediately on Windows until smooth cursor support exists.
func (s *SystemAdapter) WaitForCursorIdle(ctx context.Context) error {
	return nil
}

// CursorPosition returns the current cursor position on Windows.
func (s *SystemAdapter) CursorPosition(ctx context.Context) (image.Point, error) {
	err := ctx.Err()
	if err != nil {
		return image.Point{}, err
	}

	return cursorPosition()
}

// IsDarkMode returns true if Windows app dark mode is currently active.
func (s *SystemAdapter) IsDarkMode() bool {
	return AppsUseDarkTheme()
}

// CheckPermissions verifies accessibility permissions on Windows.
// TODO(windows): Windows UI Automation does not require explicit permission grants.
func (s *SystemAdapter) CheckPermissions(ctx context.Context) error {
	return nil
}

// IsSecureInputEnabled returns false on Windows — secure input is a macOS-only concept.
func (s *SystemAdapter) IsSecureInputEnabled() bool {
	return false
}

// ShowSecureInputNotification is a no-op on Windows — secure input is a macOS-only concept.
func (s *SystemAdapter) ShowSecureInputNotification() {}

// ShowAlert displays a native system alert on Windows.
// TODO(windows): implement using MessageBox (user32.dll) or Windows Toast Notifications.
func (s *SystemAdapter) ShowAlert(ctx context.Context, title, message string) error {
	return derrors.New(derrors.CodeNotSupported, "ShowAlert not yet implemented on windows")
}

// ShowNotification displays a lightweight notification on Windows.
// TODO(windows): implement using Windows Toast Notifications API.
func (s *SystemAdapter) ShowNotification(title, message string) {}

// Ensure SystemAdapter implements ports.SystemPort.
var _ ports.SystemPort = (*SystemAdapter)(nil)
