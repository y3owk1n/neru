//go:build windows

// SystemAdapter: the ports.SystemPort implementation for Windows.
// The package comment lives in doc.go.

//nolint:godox // TODO comments are intentional contributor guidance for unimplemented stubs.
package windows

import (
	"context"
	"image"
	"os"
	"path/filepath"

	wintray "github.com/y3owk1n/neru/internal/adapter/systray/windows"
	"github.com/y3owk1n/neru/internal/domain/geometry"
	"github.com/y3owk1n/neru/internal/ports"
)

// SystemAdapter implements ports.SystemPort for Windows.
type SystemAdapter struct {
	cursorAnimator *smoothCursorAnimator
}

// NewSystemAdapter creates a new SystemAdapter.
func NewSystemAdapter() *SystemAdapter {
	adapter := &SystemAdapter{}
	adapter.cursorAnimator = newSmoothCursorAnimator(
		adapter.currentCursorPosition,
		moveCursorTo,
	)

	return adapter
}

// PlatformLabel returns "windows".
func (s *SystemAdapter) PlatformLabel() string { return "windows" }

// Health checks the health of the Windows system adapter.
func (s *SystemAdapter) Health(ctx context.Context) error {
	return nil
}

// Capabilities returns the current Windows capability surface.
//
// Notifications are balloon tips on the tray icon, so once this process has
// started its tray loop the row follows whether the shell holds the icon: a
// headless run or a failed registration downgrades it live with the reason,
// and neru doctor says so rather than repeating the static preset. Before the
// loop starts, and in a process that never runs one, the preset stands.
func (s *SystemAdapter) Capabilities() ports.PlatformCapabilities {
	capabilities := ports.WindowsCapabilities()

	started, shown := wintray.TrayState()
	if started && !shown {
		capabilities.Notifications = ports.FeatureCapability{
			Status: ports.FeatureStatusStub,
			Detail: wintray.NoTrayIconDetail(),
		}
	}

	return capabilities
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
//
// When smooth cursor is enabled (config smooth_cursor.move_mouse_enabled) and
// the caller has not requested a bypass, the move is animated by the shared
// cursor animator, which steps SetCursorPos over time; callers that need the
// cursor settled before acting pair this with WaitForCursorIdle. Otherwise
// (bypass, disabled, or no config wired) it warps directly.
func (s *SystemAdapter) MoveCursorToPoint(
	ctx context.Context,
	point image.Point,
	bypassSmooth bool,
) error {
	err := ctx.Err()
	if err != nil {
		return err
	}

	cfg := currentWindowsConfig()
	if cfg != nil && cfg.SmoothCursor.MoveMouseEnabled && !bypassSmooth {
		s.cursorAnimator.animateTo(
			point,
			cfg.SmoothCursor.Steps,
			cfg.SmoothCursor.MaxDuration,
			cfg.SmoothCursor.DurationPerPixel,
		)

		return nil
	}

	// Stop before warping: stop() drains the animator's injection mutex, so on
	// return no animation step is in flight and none will start, and this warp
	// lands last instead of racing a stale step back to an intermediate point.
	s.cursorAnimator.stop()

	return moveCursorTo(point)
}

// MoveCursorBy applies a relative cursor move. With smooth cursor enabled
// (smooth_cursor.move_mouse_enabled) the delta extends the animator's pending
// endpoint, clamped to the active screen, and animates over the fixed
// per-move duration smooth_cursor.relative_movement_duration, matching macOS.
// Otherwise it reports handled == false so the caller keeps its
// CursorPosition + MoveCursorToPoint fallback, the instant warp Windows always
// used.
func (s *SystemAdapter) MoveCursorBy(
	ctx context.Context,
	delta image.Point,
) (bool, error) {
	cfg := currentWindowsConfig()
	if cfg == nil || !cfg.SmoothCursor.MoveMouseEnabled {
		return false, nil
	}

	// Without bounds there is nothing to clamp against, so fall through to
	// the caller's fallback, which clamps at the service layer.
	bounds, err := s.ScreenBounds(ctx)
	if err == nil {
		s.cursorAnimator.animateRelativeBy(
			delta,
			func(point image.Point) image.Point {
				return image.Point{
					X: geometry.ClampInt(point.X, bounds.Min.X, max(bounds.Max.X-1, bounds.Min.X)),
					Y: geometry.ClampInt(point.Y, bounds.Min.Y, max(bounds.Max.Y-1, bounds.Min.Y)),
				}
			},
			cfg.SmoothCursor.Steps,
			cfg.SmoothCursor.RelativeMovementDuration,
		)

		return true, nil
	}

	return false, nil
}

// WaitForCursorIdle blocks until any in-flight smooth cursor animation settles,
// or ctx is canceled. It returns immediately when no animation is active, the
// common case on the direct (non-smooth) move path.
func (s *SystemAdapter) WaitForCursorIdle(ctx context.Context) error {
	return s.cursorAnimator.wait(ctx)
}

// SettleCursor finishes any in-flight cursor animation immediately: the
// worker is stopped and the cursor warps straight to the endpoint it was
// animating toward. Action paths call this before resolving their target
// point from the cursor, so an action firing mid-animation acts at the point
// the user aimed for without paying the animation's remaining duration.
func (s *SystemAdapter) SettleCursor(ctx context.Context) error {
	s.cursorAnimator.settle()

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

// ShowAlert displays a native system alert on Windows using MessageBoxW.
func (s *SystemAdapter) ShowAlert(_ context.Context, title, message string) error {
	ShowAlert(title, message)

	return nil
}

// ShowNotification shows a balloon tip on the tray icon, which Windows 10 and
// 11 render as a toast. The tray is the anchor, so with systray.enabled off the
// call reports CodeNotSupported with that reason and the caller logs it.
func (s *SystemAdapter) ShowNotification(_ context.Context, title, message string) error {
	return wintray.ShowBalloon(title, message)
}

// CheckScreenCapturePermission reports true: Windows does not gate screen capture
// behind a permission.
func (s *SystemAdapter) CheckScreenCapturePermission(_ context.Context) bool {
	return true
}

// RequestScreenCapturePermission reports granted without prompting: Windows has no
// screen-recording permission to request.
func (s *SystemAdapter) RequestScreenCapturePermission(
	_ context.Context,
) ports.ScreenCaptureConsent {
	return ports.ScreenCaptureGranted
}

// currentCursorPosition returns the current cursor position for the animator,
// collapsing the error to a zero point. The animator samples this once per
// request to seed interpolation; a bad read only skews the glide path, never
// the final landing point (the last step lands exactly on the target).
func (s *SystemAdapter) currentCursorPosition() image.Point {
	point, err := cursorPosition()
	if err != nil {
		return image.Point{}
	}

	return point
}

// Ensure SystemAdapter implements ports.SystemPort and its optional cursor
// extensions.
var (
	_ ports.SystemPort          = (*SystemAdapter)(nil)
	_ ports.RelativeCursorMover = (*SystemAdapter)(nil)
	_ ports.CursorSettler       = (*SystemAdapter)(nil)
)
