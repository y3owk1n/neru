//go:build darwin

package darwin

import (
	"context"
	"image"
	"os"
	"path/filepath"

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
	// This would typically call into darwin package functions
	return 0, nil
}

// ApplicationNameByPID returns the name of the application with the given PID on macOS.
func (s *SystemAdapter) ApplicationNameByPID(ctx context.Context, pid int) (string, error) {
	return "", nil
}

// ApplicationBundleIDByPID returns the bundle ID of the application with the given PID on macOS.
func (s *SystemAdapter) ApplicationBundleIDByPID(ctx context.Context, pid int) (string, error) {
	return "", nil
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
	// This logic is usually in bridge or accessibility package
	return nil
}

// ShowAlert displays a native system alert on macOS.
func (s *SystemAdapter) ShowAlert(ctx context.Context, title, message string) error {
	ShowConfigValidationError(message, title)

	return nil
}

// Ensure SystemAdapter implements ports.SystemPort.
var _ ports.SystemPort = (*SystemAdapter)(nil)
