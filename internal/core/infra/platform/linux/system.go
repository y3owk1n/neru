// Package linux provides Linux-specific implementations of infrastructure components.
package linux

import (
	"context"
	"image"
	"os"
	"path/filepath"

	"github.com/y3owk1n/neru/internal/core/ports"
)

// SystemAdapter implements ports.SystemPort for Linux.
type SystemAdapter struct{}

// NewSystemAdapter creates a new SystemAdapter.
func NewSystemAdapter() *SystemAdapter {
	return &SystemAdapter{}
}

// Health checks the health of the Linux system adapter.
func (s *SystemAdapter) Health(ctx context.Context) error {
	return nil
}

// ConfigDir returns the Linux-specific configuration directory.
func (s *SystemAdapter) ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".config", "neru"), nil
}

// UserDataDir returns the Linux-specific user data directory.
func (s *SystemAdapter) UserDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".local", "share", "neru"), nil
}

// LogDir returns the Linux-specific log directory.
func (s *SystemAdapter) LogDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".local", "state", "neru", "log"), nil
}

// FocusedApplicationPID returns the PID of the currently focused application on Linux.
func (s *SystemAdapter) FocusedApplicationPID(ctx context.Context) (int, error) {
	return 0, nil
}

// ApplicationNameByPID returns the name of the application with the given PID on Linux.
func (s *SystemAdapter) ApplicationNameByPID(ctx context.Context, pid int) (string, error) {
	return "", nil
}

// ApplicationBundleIDByPID returns the application identifier (desktop ID) for Linux.
func (s *SystemAdapter) ApplicationBundleIDByPID(ctx context.Context, pid int) (string, error) {
	return "", nil
}

// ScreenBounds returns the bounds of the active screen on Linux.
func (s *SystemAdapter) ScreenBounds(ctx context.Context) (image.Rectangle, error) {
	return image.Rectangle{}, nil
}

// MoveCursorToPoint moves the mouse cursor to the specified point on Linux.
func (s *SystemAdapter) MoveCursorToPoint(
	ctx context.Context,
	point image.Point,
	bypassSmooth bool,
) error {
	return nil
}

// CursorPosition returns the current cursor position on Linux.
func (s *SystemAdapter) CursorPosition(ctx context.Context) (image.Point, error) {
	return image.Point{}, nil
}

// IsDarkMode returns true if Linux dark mode is currently active.
func (s *SystemAdapter) IsDarkMode() bool {
	return false
}

// CheckPermissions verifies accessibility permissions on Linux.
func (s *SystemAdapter) CheckPermissions(ctx context.Context) error {
	return nil
}

// IsSecureInputEnabled returns false on Linux (secure input is a macOS concept).
func (s *SystemAdapter) IsSecureInputEnabled() bool {
	return false
}

// ShowSecureInputNotification is a no-op on Linux.
func (s *SystemAdapter) ShowSecureInputNotification() {}

// ShowAlert displays a native system alert on Linux.
func (s *SystemAdapter) ShowAlert(ctx context.Context, title, message string) error {
	return nil
}

// Ensure SystemAdapter implements ports.SystemPort.
var _ ports.SystemPort = (*SystemAdapter)(nil)
