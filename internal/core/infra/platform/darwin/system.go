//go:build darwin

package darwin

import (
	"context"
	"image"
	"os"
	"path/filepath"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// SystemAdapter implements ports.SystemPort for macOS.
type SystemAdapter struct{}

// NewSystemAdapter creates a new SystemAdapter.
func NewSystemAdapter() *SystemAdapter {
	return &SystemAdapter{}
}

// Health checks the health of the macOS system adapter.
func (s *SystemAdapter) Health(ctx context.Context) error {
	return nil
}

// ConfigDir returns the macOS-specific configuration directory.
func (s *SystemAdapter) ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, "Library", "Application Support", "neru"), nil
}

// UserDataDir returns the macOS-specific user data directory.
func (s *SystemAdapter) UserDataDir() (string, error) {
	return s.ConfigDir()
}

// LogDir returns the macOS-specific log directory.
func (s *SystemAdapter) LogDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, "Library", "Logs", "neru"), nil
}

// FocusedApplicationPID returns the PID of the currently focused application on macOS.
func (s *SystemAdapter) FocusedApplicationPID(ctx context.Context) (int, error) {
	return FocusedApplicationPID()
}

// ApplicationNameByPID returns the name of the application with the given PID on macOS.
func (s *SystemAdapter) ApplicationNameByPID(ctx context.Context, pid int) (string, error) {
	return ApplicationNameByPID(pid)
}

// ApplicationBundleIDByPID returns the bundle ID of the application with the given PID on macOS.
func (s *SystemAdapter) ApplicationBundleIDByPID(ctx context.Context, pid int) (string, error) {
	return ApplicationBundleIDByPID(pid)
}

// ScreenBounds returns the bounds of the active screen on macOS.
func (s *SystemAdapter) ScreenBounds(ctx context.Context) (image.Rectangle, error) {
	return ActiveScreenBounds(), nil
}

// MoveCursorToPoint moves the mouse cursor to the specified point on macOS.
func (s *SystemAdapter) MoveCursorToPoint(
	ctx context.Context,
	point image.Point,
	bypassSmooth bool,
) error {
	MoveMouse(point, bypassSmooth)

	return nil
}

// CursorPosition returns the current cursor position on macOS.
func (s *SystemAdapter) CursorPosition(ctx context.Context) (image.Point, error) {
	return CursorPosition(), nil
}

// IsDarkMode returns true if macOS Dark Mode is currently active.
func (s *SystemAdapter) IsDarkMode() bool {
	return IsDarkMode()
}

// CheckPermissions verifies accessibility permissions on macOS.
func (s *SystemAdapter) CheckPermissions(ctx context.Context) error {
	if !CheckAccessibilityPermissions() {
		return derrors.New(
			derrors.CodeAccessibilityDenied,
			"accessibility permissions not granted - please enable in System Preferences > Privacy & Security > Accessibility",
		)
	}

	return nil
}

// IsSecureInputEnabled returns true if macOS secure input mode is currently active.
func (s *SystemAdapter) IsSecureInputEnabled() bool {
	return IsSecureInputEnabled()
}

// ShowSecureInputNotification displays a macOS notification about active secure input.
func (s *SystemAdapter) ShowSecureInputNotification() {
	ShowSecureInputNotification()
}

// ShowAlert displays a native system alert on macOS.
func (s *SystemAdapter) ShowAlert(ctx context.Context, title, message string) error {
	ShowConfigValidationError(title, message)

	return nil
}

// ShowNotification displays a lightweight toast/banner notification on macOS.
func (s *SystemAdapter) ShowNotification(title, message string) {
	ShowNotification(title, message)
}

// Ensure SystemAdapter implements ports.SystemPort.
var _ ports.SystemPort = (*SystemAdapter)(nil)
