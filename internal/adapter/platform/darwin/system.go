//go:build darwin

package darwin

import (
	"context"
	"image"
	"os"
	"path/filepath"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

// SystemAdapter implements ports.SystemPort for macOS.
type SystemAdapter struct{}

// NewSystemAdapter creates a new SystemAdapter.
func NewSystemAdapter() *SystemAdapter {
	return &SystemAdapter{}
}

// PlatformLabel returns "darwin".
func (s *SystemAdapter) PlatformLabel() string { return "darwin" }

// Health checks the health of the macOS system adapter.
func (s *SystemAdapter) Health(ctx context.Context) error {
	return nil
}

// Capabilities returns the supported macOS capabilities.
func (s *SystemAdapter) Capabilities() ports.PlatformCapabilities {
	return ports.DarwinCapabilities()
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

// ScreenBoundsByName returns the bounds of the screen with the given localized
// display name (case-insensitive) on macOS.
func (s *SystemAdapter) ScreenBoundsByName(
	ctx context.Context,
	name string,
) (image.Rectangle, bool, error) {
	bounds, found := ScreenBoundsByName(name)

	return bounds, found, nil
}

// ScreenNames returns the localized display names of all connected screens on macOS.
func (s *SystemAdapter) ScreenNames(ctx context.Context) ([]string, error) {
	return ScreenNames(), nil
}

// FocusedWindowBounds returns the bounds of the currently focused window on macOS.
func (s *SystemAdapter) FocusedWindowBounds(
	ctx context.Context,
) (image.Rectangle, bool, error) {
	bounds, found := FocusedWindowBounds()

	return bounds, found, nil
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

// MoveCursorBy animates a relative cursor move when smooth cursor is enabled
// (smooth_cursor.move_mouse_enabled). It reports handled == false when it is
// disabled, sending the caller to its CursorPosition + MoveCursorToPoint
// fallback — the instant warp that was always used before relative moves
// gained animation.
func (s *SystemAdapter) MoveCursorBy(
	ctx context.Context,
	delta image.Point,
) (bool, error) {
	return MoveMouseRelativeSmooth(delta), nil
}

// MoveCursorInstantly posts one cursor move without animating or waiting
// (ports.InstantCursorMover).
func (s *SystemAdapter) MoveCursorInstantly(ctx context.Context, point image.Point) error {
	PostMouseMove(point)

	return nil
}

// WaitForCursorIdle blocks until any in-flight cursor movement animation settles.
func (s *SystemAdapter) WaitForCursorIdle(ctx context.Context) error {
	return cursorAnimator.wait(ctx)
}

// CursorPosition returns the current cursor position on macOS.
//
// This is a pure read: an in-flight animation reports its mid-animation
// position, which is what pollers like the mode-indicator follower need.
// Callers resolving an action's target point use SettleCursor first.
func (s *SystemAdapter) CursorPosition(ctx context.Context) (image.Point, error) {
	return CursorPosition(), nil
}

// SettleCursor finishes any in-flight cursor animation immediately: the
// worker is stopped and the cursor warps straight to the endpoint it was
// animating toward. Action paths call this before resolving their target
// point from the cursor, so an action firing mid-animation (e.g. a click
// right after an animated relative move) acts at the point the user aimed
// for — without paying the animation's remaining duration in latency, and
// without disturbing pollers that merely observe the cursor.
func (s *SystemAdapter) SettleCursor(ctx context.Context) error {
	cursorAnimator.settle()

	return nil
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
//
// The UserNotifications path is fire-and-forget, so this returns as soon as
// the request is handed to the notification center; there is no failure to
// report at this point and it always reports success.
func (s *SystemAdapter) ShowNotification(_ context.Context, title, message string) error {
	ShowNotification(title, message)

	return nil
}

// CheckScreenCapturePermission reports whether macOS screen recording is
// permitted, without prompting.
func (s *SystemAdapter) CheckScreenCapturePermission(_ context.Context) bool {
	return CheckScreenCapturePermissions()
}

// RequestScreenCapturePermission shows the macOS screen-recording guidance and
// reports the user's choice.
func (s *SystemAdapter) RequestScreenCapturePermission(
	_ context.Context,
) ports.ScreenCaptureConsent {
	switch ShowScreenCapturePermissionAlert() {
	case screenCaptureAlertQuit:
		return ports.ScreenCaptureQuit
	case screenCaptureAlertCancel:
		return ports.ScreenCaptureCanceled
	default:
		return ports.ScreenCaptureGranted
	}
}

// Ensure SystemAdapter implements ports.SystemPort.
var _ ports.SystemPort = (*SystemAdapter)(nil)

// Ensure SystemAdapter opts into relative cursor movement (animated relative
// moves when smooth cursor is enabled).
var _ ports.RelativeCursorMover = (*SystemAdapter)(nil)

// SystemAdapter posts single moves without waiting for the held-key glide.
var _ ports.InstantCursorMover = (*SystemAdapter)(nil)

// Ensure SystemAdapter opts into settling in-flight cursor animations before
// position-dependent actions.
var _ ports.CursorSettler = (*SystemAdapter)(nil)
