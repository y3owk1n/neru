// Package linux provides Linux-specific implementations of infrastructure components.
//
// Most methods currently return CodeNotSupported because Linux support is a
// work-in-progress. Contributors should replace each stub with a real
// implementation and remove the CodeNotSupported return when done.
// See docs/ARCHITECTURE_CROSS_PLATFORM.md for the contribution guide.
package linux

import (
	"context"
	"image"
	"os"
	"path/filepath"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
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
// TODO(linux): implement using AT-SPI or /proc filesystem.
func (s *SystemAdapter) FocusedApplicationPID(ctx context.Context) (int, error) {
	return 0, derrors.New(derrors.CodeNotSupported, "FocusedApplicationPID not yet implemented on linux")
}

// ApplicationNameByPID returns the name of the application with the given PID on Linux.
// TODO(linux): implement using /proc/<pid>/comm or AT-SPI.
func (s *SystemAdapter) ApplicationNameByPID(ctx context.Context, pid int) (string, error) {
	return "", derrors.New(derrors.CodeNotSupported, "ApplicationNameByPID not yet implemented on linux")
}

// ApplicationBundleIDByPID returns the application identifier (desktop ID) for Linux.
// TODO(linux): implement using /proc/<pid>/cmdline + .desktop file lookup.
func (s *SystemAdapter) ApplicationBundleIDByPID(ctx context.Context, pid int) (string, error) {
	return "", derrors.New(derrors.CodeNotSupported, "ApplicationBundleIDByPID not yet implemented on linux")
}

// ScreenBounds returns the bounds of the active screen on Linux.
// TODO(linux): implement using XRandR or Wayland display protocol.
func (s *SystemAdapter) ScreenBounds(ctx context.Context) (image.Rectangle, error) {
	return image.Rectangle{}, derrors.New(derrors.CodeNotSupported, "ScreenBounds not yet implemented on linux")
}

// MoveCursorToPoint moves the mouse cursor to the specified point on Linux.
// TODO(linux): implement using XTest (X11) or libinput (Wayland).
func (s *SystemAdapter) MoveCursorToPoint(
	ctx context.Context,
	point image.Point,
	bypassSmooth bool,
) error {
	return derrors.New(derrors.CodeNotSupported, "MoveCursorToPoint not yet implemented on linux")
}

// CursorPosition returns the current cursor position on Linux.
// TODO(linux): implement using XQueryPointer (X11) or Wayland pointer protocol.
func (s *SystemAdapter) CursorPosition(ctx context.Context) (image.Point, error) {
	return image.Point{}, derrors.New(derrors.CodeNotSupported, "CursorPosition not yet implemented on linux")
}

// IsDarkMode returns true if Linux dark mode is currently active.
// TODO(linux): implement using org.freedesktop.appearance color-scheme D-Bus property.
func (s *SystemAdapter) IsDarkMode() bool {
	return false
}

// CheckPermissions verifies accessibility permissions on Linux.
// Linux uses AT-SPI which does not require explicit permission grants in most distros.
func (s *SystemAdapter) CheckPermissions(ctx context.Context) error {
	return nil
}

// IsSecureInputEnabled returns false on Linux — secure input is a macOS-only concept.
func (s *SystemAdapter) IsSecureInputEnabled() bool {
	return false
}

// ShowSecureInputNotification is a no-op on Linux — secure input is a macOS-only concept.
func (s *SystemAdapter) ShowSecureInputNotification() {}

// ShowAlert displays a native system alert on Linux.
// TODO(linux): implement using libnotify, zenity, or kdialog.
func (s *SystemAdapter) ShowAlert(ctx context.Context, title, message string) error {
	return derrors.New(derrors.CodeNotSupported, "ShowAlert not yet implemented on linux")
}

// ShowNotification displays a lightweight notification on Linux.
// TODO(linux): implement using org.freedesktop.Notifications D-Bus interface.
func (s *SystemAdapter) ShowNotification(title, message string) {}

// Ensure SystemAdapter implements ports.SystemPort.
var _ ports.SystemPort = (*SystemAdapter)(nil)
