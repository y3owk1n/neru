// Package windows provides Windows-specific implementations of infrastructure components.
//
// Most methods currently return CodeNotSupported because Windows support is a
// work-in-progress. Contributors should replace each stub with a real
// implementation and remove the CodeNotSupported return when done.
// See docs/ARCHITECTURE_CROSS_PLATFORM.md for the contribution guide.
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

// FocusedApplicationPID returns the PID of the currently focused application on Windows.
// TODO(windows): implement using GetForegroundWindow + GetWindowThreadProcessId.
func (s *SystemAdapter) FocusedApplicationPID(ctx context.Context) (int, error) {
	return 0, derrors.New(
		derrors.CodeNotSupported,
		"FocusedApplicationPID not yet implemented on windows",
	)
}

// ApplicationNameByPID returns the name of the application with the given PID on Windows.
// TODO(windows): implement using OpenProcess + QueryFullProcessImageName.
func (s *SystemAdapter) ApplicationNameByPID(ctx context.Context, pid int) (string, error) {
	return "", derrors.New(
		derrors.CodeNotSupported,
		"ApplicationNameByPID not yet implemented on windows",
	)
}

// ApplicationBundleIDByPID returns the application identifier for Windows.
// TODO(windows): Windows does not have bundle IDs; use executable path or AppUserModelID.
func (s *SystemAdapter) ApplicationBundleIDByPID(ctx context.Context, pid int) (string, error) {
	return "", derrors.New(
		derrors.CodeNotSupported,
		"ApplicationBundleIDByPID not yet implemented on windows",
	)
}

// ScreenBounds returns the bounds of the active screen on Windows.
// TODO(windows): implement using MonitorFromPoint + GetMonitorInfo.
func (s *SystemAdapter) ScreenBounds(ctx context.Context) (image.Rectangle, error) {
	return image.Rectangle{}, derrors.New(
		derrors.CodeNotSupported,
		"ScreenBounds not yet implemented on windows",
	)
}

// MoveCursorToPoint moves the mouse cursor to the specified point on Windows.
// TODO(windows): implement using SetCursorPos (user32.dll).
func (s *SystemAdapter) MoveCursorToPoint(
	ctx context.Context,
	point image.Point,
	bypassSmooth bool,
) error {
	return derrors.New(derrors.CodeNotSupported, "MoveCursorToPoint not yet implemented on windows")
}

// CursorPosition returns the current cursor position on Windows.
// TODO(windows): implement using GetCursorPos (user32.dll).
func (s *SystemAdapter) CursorPosition(ctx context.Context) (image.Point, error) {
	return image.Point{}, derrors.New(
		derrors.CodeNotSupported,
		"CursorPosition not yet implemented on windows",
	)
}

// IsDarkMode returns true if Windows dark mode is currently active.
// TODO(windows): implement using registry key
// HKCU\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize\AppsUseLightTheme.
func (s *SystemAdapter) IsDarkMode() bool {
	return false
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
