//go:build linux

//nolint:godox // TODO comments are intentional contributor guidance for unimplemented stubs.
package linux

import (
	"bufio"
	"context"
	"image"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/ports"
)

const (
	backendX11            = "x11"
	backendWaylandWlroots = "wayland-wlroots"
	backendWaylandKDE     = "wayland-kde"
)

// SystemAdapter is a Linux system adapter.
type SystemAdapter struct {
	backend        string
	cursorAnimator *smoothCursorAnimator
}

// NewSystemAdapter creates a new SystemAdapter.
func NewSystemAdapter(backend string) *SystemAdapter {
	adapter := &SystemAdapter{backend: backend}
	adapter.cursorAnimator = newSmoothCursorAnimator(
		adapter.currentCursorPosition,
		adapter.moveCursorDirect,
	)

	return adapter
}

// PlatformLabel returns "linux/<backend>" (e.g. "linux/x11", "linux/wayland-kde").
// Uses the cached backend field set at construction time — no I/O or live probes.
func (s *SystemAdapter) PlatformLabel() string {
	if s.backend != "" {
		return "linux/" + s.backend
	}

	return "linux"
}

// Health checks the health of the Linux system adapter.
func (s *SystemAdapter) Health(ctx context.Context) error {
	return nil
}

// Capabilities returns the current Linux capability surface.
//
// The dark-mode capability is live-probed: if a working source can be reached
// the Detail field carries the current state ("dark" / "light" / "no
// preference") plus the source name; if none of the sources work the status
// is downgraded to stub with a fix-it hint. This is more useful than a
// static "supported" claim because the user's actual question when running
// `neru doctor` is "is dark mode being detected right now?", not "does the
// code path exist on Linux?".
func (s *SystemAdapter) Capabilities() ports.PlatformCapabilities {
	capabilities := ports.LinuxCapabilities()

	if s.backend != "" {
		capabilities.Platform = "linux/" + s.backend
	}

	value, source, ok := darkModePreference()
	capabilities.DarkModeDetection = darkModeCapability(value, source, ok)

	return capabilities
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
//
// X11 reads _NET_WM_PID directly. Wayland (wlroots + KDE) has no protocol that
// exposes another client's PID, so it resolves the focused window's app_id via
// wlr-foreign-toplevel-management and best-effort matches it against /proc;
// unmatched app_ids return CodeNotSupported rather than a fabricated PID.
func (s *SystemAdapter) FocusedApplicationPID(ctx context.Context) (int, error) {
	if s.backend == backendX11 {
		return x11FocusedApplicationPID()
	}

	if s.waylandUsesWlrClientStack() {
		return waylandFocusedApplicationPID()
	}

	return 0, derrors.New(
		derrors.CodeNotSupported,
		"FocusedApplicationPID not yet implemented on linux backend "+s.backend,
	)
}

// ApplicationNameByPID returns the name of the application with the given PID on Linux.
// Process inspection reads procfs, so it is display-server agnostic and works on
// every Linux backend (X11 and all Wayland compositors).
func (s *SystemAdapter) ApplicationNameByPID(ctx context.Context, pid int) (string, error) {
	return linuxApplicationNameByPID(pid)
}

// ApplicationBundleIDByPID returns the application identifier (desktop ID) for Linux.
// Derived from procfs (argv[0]), so it is display-server agnostic and works on
// every Linux backend (X11 and all Wayland compositors).
func (s *SystemAdapter) ApplicationBundleIDByPID(ctx context.Context, pid int) (string, error) {
	return linuxApplicationBundleIDByPID(pid)
}

// ScreenBounds returns the bounds of the active screen on Linux.
// TODO(linux): implement using XRandR or Wayland display protocol.
func (s *SystemAdapter) ScreenBounds(ctx context.Context) (image.Rectangle, error) {
	if s.backend == backendX11 {
		return x11ActiveScreenBounds()
	}

	if s.waylandUsesWlrClientStack() {
		return wlrootsScreenBounds()
	}

	return image.Rectangle{}, derrors.New(
		derrors.CodeNotSupported,
		"ScreenBounds not yet implemented on linux backend "+s.backend,
	)
}

// ScreenBoundsByName returns the bounds of the screen with the given name on Linux.
// TODO(linux): implement using XRandR or Wayland output protocol.
func (s *SystemAdapter) ScreenBoundsByName(
	ctx context.Context,
	name string,
) (image.Rectangle, bool, error) {
	if s.backend == backendX11 {
		return x11ScreenBoundsByName(name)
	}

	if s.waylandUsesWlrClientStack() {
		return wlrootsScreenBoundsByName(name)
	}

	return image.Rectangle{}, false, derrors.New(
		derrors.CodeNotSupported,
		"ScreenBoundsByName not yet implemented on linux backend "+s.backend,
	)
}

// ScreenNames returns the display names of all connected screens on Linux.
// TODO(linux): implement using XRandR or Wayland output protocol.
func (s *SystemAdapter) ScreenNames(ctx context.Context) ([]string, error) {
	if s.backend == backendX11 {
		return x11ScreenNames()
	}

	if s.waylandUsesWlrClientStack() {
		return wlrootsScreenNames()
	}

	return nil, derrors.New(
		derrors.CodeNotSupported,
		"ScreenNames not yet implemented on linux backend "+s.backend,
	)
}

// FocusedWindowBounds returns the global bounds of the currently focused window
// on Linux, used to constrain hint/vision detection to the active window's
// monitor (matching darwin). X11 reads _NET_ACTIVE_WINDOW geometry directly.
// Wayland has no protocol exposing another client's on-screen geometry, so it
// queries the running wlroots-family compositor's IPC (niri/Sway/Hyprland);
// KWin and GNOME return not-found and callers fall back to the active screen.
//
// found=false with a nil error means "no focused window bounds available" — a
// normal fallback, not an error.
func (s *SystemAdapter) FocusedWindowBounds(
	ctx context.Context,
) (image.Rectangle, bool, error) {
	if s.backend == backendX11 {
		return x11FocusedWindowBounds()
	}

	if s.waylandUsesWlrClientStack() {
		return waylandFocusedWindowBounds()
	}

	return image.Rectangle{}, false, derrors.New(
		derrors.CodeNotSupported,
		"FocusedWindowBounds not yet implemented on linux backend "+s.backend,
	)
}

// MoveCursorToPoint moves the mouse cursor to the specified point on Linux.
//
// When smooth cursor is enabled (config smooth_cursor.move_mouse_enabled) and
// the caller has not requested a bypass, the move is animated by the shared
// cursor animator, which steps moveCursorDirect over time; callers that need
// the cursor settled before acting pair this with WaitForCursorIdle. Otherwise
// (bypass, disabled, or no config wired) it warps directly. The animator is a
// no-op wrapper on the direct path, so behavior is unchanged unless the user
// opts into smooth cursor. Backend routing lives in moveCursorDirect.
func (s *SystemAdapter) MoveCursorToPoint(
	ctx context.Context,
	point image.Point,
	bypassSmooth bool,
) error {
	cfg := currentLinuxConfig()
	if cfg != nil && cfg.SmoothCursor.MoveMouseEnabled && !bypassSmooth {
		s.cursorAnimator.animateTo(
			point,
			cfg.SmoothCursor.Steps,
			cfg.SmoothCursor.MaxDuration,
			cfg.SmoothCursor.DurationPerPixel,
		)

		return nil
	}

	// Stop before warping: the animation worker must be canceled first so a
	// pending tween step cannot override the direct warp. stop() drains the
	// animator's injection mutex, so on return no animation step is in flight
	// and none will start — this warp lands last, not racing a stale step back
	// to an intermediate point. (Which of two distinct concurrent callers wins
	// is still last-writer-wins, as on macOS.)
	s.cursorAnimator.stop()

	return s.moveCursorDirect(point)
}

// MoveCursorBy uses native relative motion when the Wayland backend supports it.
// The bool result reports whether the request was handled, allowing callers to
// retain their absolute-position fallback on X11 and unsupported backends.
func (s *SystemAdapter) MoveCursorBy(
	ctx context.Context,
	delta image.Point,
) (bool, error) {
	if s.backend == backendWaylandWlroots {
		return true, wlrootsMoveCursorBy(delta)
	}

	return false, nil
}

// WaitForCursorIdle blocks until any in-flight smooth cursor animation settles,
// or ctx is canceled. It returns immediately when no animation is active — the
// common case on the direct (non-smooth) move path.
func (s *SystemAdapter) WaitForCursorIdle(ctx context.Context) error {
	return s.cursorAnimator.wait(ctx)
}

// CursorPosition returns the current cursor position on Linux.
// TODO(linux): implement using XQueryPointer (X11) or Wayland pointer protocol.
func (s *SystemAdapter) CursorPosition(ctx context.Context) (image.Point, error) {
	if s.backend == backendX11 {
		return x11CursorPosition()
	}

	if s.waylandUsesWlrClientStack() {
		return waylandCursorPosition()
	}

	return image.Point{}, derrors.New(
		derrors.CodeNotSupported,
		"CursorPosition not yet implemented on linux backend "+s.backend,
	)
}

// SyncCursorPosition refreshes Wayland's client-side cursor cache from the
// compositor when possible. X11 and macOS query the OS directly on every
// CursorPosition call; Wayland does not expose a global pointer query, so the
// Wayland backend briefly maps transparent layer-shell surfaces and captures
// wl_pointer.enter coordinates before a mode uses the cached position.
func (s *SystemAdapter) SyncCursorPosition(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if s.waylandUsesWlrClientStack() {
		return waylandRefreshCursorPosition()
	}

	return nil
}

// IsDarkMode returns true if Linux dark mode is currently active. See
// darkModePreference for source ordering and semantics.
func (s *SystemAdapter) IsDarkMode() bool {
	value, _, ok := darkModePreference()

	return ok && value == colorSchemeDark
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

// CheckScreenCapturePermission reports true: Linux does not gate screen capture
// behind a permission.
func (s *SystemAdapter) CheckScreenCapturePermission(_ context.Context) bool {
	return true
}

// RequestScreenCapturePermission reports granted without prompting: Linux has no
// screen-recording permission to request.
func (s *SystemAdapter) RequestScreenCapturePermission(
	_ context.Context,
) ports.ScreenCaptureConsent {
	return ports.ScreenCaptureGranted
}

// moveCursorDirect performs a single instantaneous cursor warp, routing to the
// backend-specific injector. It is the shared sink for both the direct move
// path and each step of the smooth animator.
func (s *SystemAdapter) moveCursorDirect(point image.Point) error {
	if s.backend == backendX11 {
		return x11MoveCursorToPoint(point)
	}

	if s.waylandUsesWlrClientStack() {
		// Route through the Wayland input dispatcher so KDE (no virtual
		// pointer) uses libei while wlroots compositors use the native path.
		return waylandMoveCursorToPoint(point)
	}

	return derrors.New(derrors.CodeNotSupported, "MoveCursorToPoint not yet implemented on linux")
}

// currentCursorPosition returns the current cursor position for the animator,
// collapsing the error to a zero point. The animator samples this once per
// request to seed interpolation; a bad read only skews the glide path, never
// the final landing point (the last step lands exactly on the target).
func (s *SystemAdapter) currentCursorPosition() image.Point {
	point, err := s.CursorPosition(context.Background())
	if err != nil {
		return image.Point{}
	}

	return point
}

// waylandUsesWlrClientStack is true when the session uses the same Wayland
// client protocols as wlroots (layer shell, xdg-output, virtual pointer, etc.).
// KDE Plasma's KWin implements these for third-party clients; GNOME does not.
func (s *SystemAdapter) waylandUsesWlrClientStack() bool {
	return s.backend == backendWaylandWlroots || s.backend == backendWaylandKDE
}

// Ensure SystemAdapter implements ports.SystemPort.
var _ ports.SystemPort = (*SystemAdapter)(nil)

// Ensure SystemAdapter keeps satisfying the optional SystemPort extensions it
// opts into. Callers reach these by type assertion, so a signature drift would
// otherwise silently downgrade Linux to the generic fallback path instead of
// failing to compile.
var (
	_ ports.RelativeCursorMover = (*SystemAdapter)(nil)
	_ ports.CursorSynchronizer  = (*SystemAdapter)(nil)
)

// darkModeSource names which input produced a color-scheme value.
type darkModeSource string

const (
	darkModeSourcePortal     darkModeSource = "xdg-portal"
	darkModeSourceKDEGlobals darkModeSource = "kdeglobals"
)

// freedesktop "color-scheme" enum (org.freedesktop.appearance).
const (
	colorSchemeNoPreference = 0
	colorSchemeDark         = 1
	colorSchemeLight        = 2
)

// darkModePortalTimeout caps the busctl call. The portal call is normally
// sub-millisecond on a healthy session bus; 250ms gives us ample margin
// without blocking `neru doctor` if the portal is wedged.
const darkModePortalTimeout = 250 * time.Millisecond

// portalBusctlMinFields is the minimum token count in a successful busctl
// Settings.Read response ("v v u N").
const portalBusctlMinFields = 4

// darkModePreference returns the active freedesktop color-scheme preference.
//
// Sources are tried in order:
//  1. The xdg-desktop-portal Settings.Read interface (works on GNOME, KDE
//     when xdg-desktop-portal-kde is installed, and any wlroots compositor
//     where xdg-desktop-portal-gtk is the fallback responder).
//  2. ~/.config/kdeglobals [General] ColorScheme — covers vanilla KDE Plasma
//     installs that haven't installed xdg-desktop-portal-kde and where the
//     gtk-portal fallback returns nothing useful.
//
// Returns ok=false when no source could be queried (e.g. busctl missing AND
// no kdeglobals on disk). Callers should treat that as "we don't know" rather
// than "light mode".
func darkModePreference() (int, darkModeSource, bool) {
	if value, ok := readPortalColorScheme(); ok {
		return value, darkModeSourcePortal, true
	}

	if value, ok := readKDEColorScheme(); ok {
		return value, darkModeSourceKDEGlobals, true
	}

	return -1, "", false
}

// readPortalColorScheme queries the xdg-desktop-portal Settings interface.
//
// The portal's Settings.Read returns a variant-of-variant containing a
// uint32: 0 = no preference, 1 = prefer dark, 2 = prefer light. busctl
// formats that as e.g. "v v u 1"; we take the trailing token.
//
// busctl's --quiet flag suppresses the method-call return value (not just
// bus chatter), so we deliberately do NOT pass it.
func readPortalColorScheme() (int, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), darkModePortalTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "busctl",
		"--user", "call",
		"org.freedesktop.portal.Desktop",
		"/org/freedesktop/portal/desktop",
		"org.freedesktop.portal.Settings",
		"Read", "ss",
		"org.freedesktop.appearance", "color-scheme",
	).Output()
	if err != nil {
		return 0, false
	}

	fields := strings.Fields(string(out))
	// Expected busctl format: "v v u N" — at least 4 tokens with the uint32
	// value as the last field. Fewer tokens means something unexpected was
	// returned (busctl exited 0 with prose we don't recognize), so we must
	// not treat the trailing token as a color-scheme value.
	if len(fields) < portalBusctlMinFields {
		return 0, false
	}

	value, err := strconv.Atoi(fields[len(fields)-1])
	if err != nil {
		return 0, false
	}

	if value < colorSchemeNoPreference || value > colorSchemeLight {
		return 0, false
	}

	return value, true
}

// readKDEColorScheme reads ~/.config/kdeglobals (and the kdedefaults variant)
// and infers a color-scheme value from the [General] ColorScheme key. Plasma
// scheme names containing "dark" (case-insensitive) — BreezeDark, OxygenDark,
// custom *Dark schemes — map to colorSchemeDark; everything else to
// colorSchemeLight. Returns ok=false when neither file exists or the key is
// missing, so the caller can fall through to "unknown".
func readKDEColorScheme() (int, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, false
	}

	candidates := []string{
		filepath.Join(home, ".config", "kdeglobals"),
		filepath.Join(home, ".config", "kdedefaults", "kdeglobals"),
	}

	for _, candidate := range candidates {
		file, err := os.Open(candidate)
		if err != nil {
			continue
		}

		scheme := scanINIValue(file, "General", "ColorScheme")
		_ = file.Close()

		if scheme == "" {
			continue
		}

		if strings.Contains(strings.ToLower(scheme), "dark") {
			return colorSchemeDark, true
		}

		return colorSchemeLight, true
	}

	return 0, false
}

// scanINIValue is a minimal INI-section/key reader. Used only for the
// kdeglobals lookups above so we don't pull in a full INI parser dependency.
// Kept generic on (section, key) rather than hardcoding "General" /
// "ColorScheme" to keep the parsing logic and the dark-mode policy decoupled
// (the tests exercise both axes).
func scanINIValue(r io.Reader, section, key string) string {
	scanner := bufio.NewScanner(r)
	sectionHeader := "[" + section + "]"
	keyPrefix := key + "="

	inSection := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inSection = (line == sectionHeader)

			continue
		}

		if inSection && strings.HasPrefix(line, keyPrefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, keyPrefix))
		}
	}

	return ""
}

// darkModeCapability builds a FeatureCapability describing the current
// dark-mode state for surfacing through `neru doctor` / IPC. When ok=false
// the capability is downgraded to stub with a fix-it hint, since "we have a
// function that returns false" is misleading -- the user almost certainly
// has a real preference set somewhere we just can't see.
func darkModeCapability(value int, source darkModeSource, ok bool) ports.FeatureCapability {
	if !ok {
		return ports.FeatureCapability{
			Status: ports.FeatureStatusStub,
			Detail: "no dark-mode source reachable; install xdg-desktop-portal-{gnome,kde} or set ~/.config/kdeglobals [General] ColorScheme",
		}
	}

	var label string

	switch value {
	case colorSchemeDark:
		label = "dark"
	case colorSchemeLight:
		label = "light"
	case colorSchemeNoPreference:
		label = "no preference"
	default:
		label = "unknown"
	}

	return ports.FeatureCapability{
		Status: ports.FeatureStatusSupported,
		Detail: "current state: " + label + " (source=" + string(source) + ")",
	}
}
