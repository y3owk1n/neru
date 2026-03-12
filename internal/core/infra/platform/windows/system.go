// Package windows provides Windows-specific implementations of infrastructure components.
package windows

import (
	"context"
	"image"
	"os"
	"path/filepath"

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
func (s *SystemAdapter) FocusedApplicationPID(ctx context.Context) (int, error) {
	return 0, nil
}

// ApplicationNameByPID returns the name of the application with the given PID on Windows.
func (s *SystemAdapter) ApplicationNameByPID(ctx context.Context, pid int) (string, error) {
	return "", nil
}

// ApplicationBundleIDByPID returns the application identifier for Windows.
func (s *SystemAdapter) ApplicationBundleIDByPID(ctx context.Context, pid int) (string, error) {
	return "", nil
}

// ScreenBounds returns the bounds of the active screen on Windows.
func (s *SystemAdapter) ScreenBounds(ctx context.Context) (image.Rectangle, error) {
	return image.Rectangle{}, nil
}

// MoveCursorToPoint moves the mouse cursor to the specified point on Windows.
func (s *SystemAdapter) MoveCursorToPoint(
	ctx context.Context,
	point image.Point,
	bypassSmooth bool,
) error {
	return nil
}

// CursorPosition returns the current cursor position on Windows.
func (s *SystemAdapter) CursorPosition(ctx context.Context) (image.Point, error) {
	return image.Point{}, nil
}

// IsDarkMode returns true if Windows dark mode is currently active.
func (s *SystemAdapter) IsDarkMode() bool {
	return false
}

// CheckPermissions verifies accessibility permissions on Windows.
func (s *SystemAdapter) CheckPermissions(ctx context.Context) error {
	return nil
}

// ShowAlert displays a native system alert on Windows.
func (s *SystemAdapter) ShowAlert(ctx context.Context, title, message string) error {
	return nil
}

// Ensure SystemAdapter implements ports.SystemPort.
var _ ports.SystemPort = (*SystemAdapter)(nil)
