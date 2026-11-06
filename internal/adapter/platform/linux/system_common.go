//go:build linux

package linux

import (
	"bufio"
	"context"
	"errors"
	"image"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/platform/gnomeshell"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/geometry"
	"github.com/y3owk1n/neru/internal/ports"
)

const (
	backendX11            = "x11"
	backendWaylandWlroots = "wayland-wlroots"
	backendWaylandKDE     = "wayland-kde"
	backendWaylandCOSMIC  = "wayland-cosmic"
	backendWaylandGNOME   = "wayland-gnome"
	// backendUnknown mirrors platform.LinuxBackend.String() for BackendUnknown.
	// It is duplicated because platform's factory imports this package.
	backendUnknown = "unknown"

	// uinputDevicePath is the node the scroll wheel and key feed are created on.
	uinputDevicePath = "/dev/uinput"
)

// SystemAdapter is a Linux system adapter.
type SystemAdapter struct {
	backend          string
	cursorAnimator   *smoothCursorAnimator
	relativeAnimator *relativeCursorAnimator
	probes           *capabilityProbes
}

// NewSystemAdapter creates a new SystemAdapter.
func NewSystemAdapter(backend string) *SystemAdapter {
	setTokenDesktop(backend)

	adapter := &SystemAdapter{backend: backend, probes: newCapabilityProbes()}
	adapter.cursorAnimator = newSmoothCursorAnimator(
		adapter.currentCursorPosition,
		adapter.moveCursorDirect,
	)
	// Only wlroots drives the delta-drain animator. Constructing it always keeps
	// every method a safe no-op elsewhere.
	adapter.relativeAnimator = newRelativeCursorAnimator(wlrootsMoveCursorBy)

	warmFocusedWindowSource(backend)

	return adapter
}

// PlatformLabel returns "linux/<backend>" from the backend cached at
// construction, with no I/O or live probes.
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

// Capabilities returns the current Linux capability surface. Dark mode,
// notifications, screen, cursor and process are live-probed, because `neru
// doctor` answers "does this work right now?", not "does the code path exist?".
func (s *SystemAdapter) Capabilities() ports.PlatformCapabilities {
	capabilities := ports.LinuxCapabilities()

	if s.backend != "" {
		capabilities.Platform = "linux/" + s.backend
	}

	value, source, ok := darkModePreference()
	capabilities.DarkModeDetection = darkModeCapability(value, source, ok)

	// Whether a notification shows depends on a daemon in the session.
	capabilities.Notifications = s.notificationCapability(capabilities.Notifications)

	// The static preset is wrong for CGO_ENABLED=0, compositors without the
	// wlroots stack, and unopenable displays. Each probe uses the same read-only
	// call the capability describes, so doctor agrees with what a caller sees.
	capabilities.Process = s.probedCapability("focused-app inspection", capabilities.Process,
		func() error {
			_, err := s.FocusedApplicationPID(context.Background())

			return err
		})
	// On GNOME the watcher's source is the Neru GNOME Shell extension, and
	// without it the watcher reports nothing, which doctor has to say.
	if s.backend == backendWaylandGNOME {
		capabilities.AppWatcher = gnomeAppWatcherCapability()
	}

	// The hint-search field needs cgo, and on Wayland a layer shell, which
	// GNOME does not have. A build without cgo, GNOME and an unknown session
	// read the search from the key stream.
	if !nativeBackendsCompiledIn || s.backend == backendWaylandGNOME ||
		s.backend == backendUnknown {
		capabilities.TextInput = ports.FeatureCapability{
			Status: ports.FeatureStatusStub,
			Detail: "no native hint-search field in this session: the query is read from " +
				"the event tap's key stream, so dead keys and input-method composition " +
				"do not reach it",
		}
	}

	capabilities.Screen = s.probedCapability("screen enumeration", capabilities.Screen,
		func() error {
			_, err := s.ScreenBounds(context.Background())

			return err
		})
	capabilities.Cursor = s.probedCapability("cursor tracking", capabilities.Cursor,
		func() error {
			_, err := s.CursorPosition(context.Background())

			return err
		})

	// Wayland scroll can lose its fast path while still returning nil. The
	// uinput wheel falls back to the virtual pointer, which Chromium and
	// Electron on Hyprland ignore. The probe creates and destroys the same
	// wheel device, exercising every ioctl the real path runs.
	if s.waylandUsesWlrClientStack() {
		capabilities.Scroll = scrollCapability(capabilities.Scroll, uinputScrollDeviceError())
	}

	return capabilities
}

// scrollCapability downgrades the declared scroll capability when the uinput
// wheel device cannot be opened, naming the fix.
func scrollCapability(declared ports.FeatureCapability, uinputErr error) ports.FeatureCapability {
	if uinputErr == nil {
		return declared
	}

	return ports.FeatureCapability{
		Status: ports.FeatureStatusStub,
		Detail: "scroll injection is falling back to the compositor virtual pointer, " +
			"which Chromium and Electron apps on Hyprland ignore: " + uinputErr.Error() +
			". Grant write access to " + uinputDevicePath + " (udev rule " +
			`KERNEL=="uinput", GROUP="input", MODE="0660"` +
			", then reload udev or reboot); see docs/guide/linux.md",
	}
}

// xdgDir resolves an XDG base directory. The variable wins when absolute (the
// spec says to ignore relative values), else the default under $HOME.
func xdgDir(envVar string, defaultParts ...string) (string, error) {
	if dir := os.Getenv(envVar); filepath.IsAbs(dir) {
		return dir, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(append([]string{home}, defaultParts...)...), nil
}

// ConfigDir returns the Linux-specific configuration directory,
// honoring $XDG_CONFIG_HOME.
func (s *SystemAdapter) ConfigDir() (string, error) {
	base, err := xdgDir("XDG_CONFIG_HOME", ".config")
	if err != nil {
		return "", err
	}

	return filepath.Join(base, "neru"), nil
}

// UserDataDir returns the Linux-specific user data directory,
// honoring $XDG_DATA_HOME.
func (s *SystemAdapter) UserDataDir() (string, error) {
	base, err := xdgDir("XDG_DATA_HOME", ".local", "share")
	if err != nil {
		return "", err
	}

	return filepath.Join(base, "neru"), nil
}

// LogDir returns the Linux-specific log directory,
// honoring $XDG_STATE_HOME.
func (s *SystemAdapter) LogDir() (string, error) {
	base, err := xdgDir("XDG_STATE_HOME", ".local", "state")
	if err != nil {
		return "", err
	}

	return filepath.Join(base, "neru", "log"), nil
}

// FocusedApplicationPID returns the PID of the focused application. X11 reads
// _NET_WM_PID. GNOME reads the Neru Shell extension. Elsewhere Wayland exposes
// no client PIDs, so the focused app_id from wlr-foreign-toplevel-management is
// matched against /proc. An unmatched app_id returns CodeNotSupported rather
// than a fabricated PID.
func (s *SystemAdapter) FocusedApplicationPID(ctx context.Context) (int, error) {
	if s.backend == backendX11 {
		return x11FocusedApplicationPID()
	}

	// GNOME shares the wlr client, but Mutter offers it no toplevel protocol.
	// The Neru GNOME Shell extension is the source there.
	if s.backend == backendWaylandGNOME {
		return gnomeFocusedApplicationPID()
	}

	if s.waylandUsesWlrClientStack() {
		return waylandFocusedApplicationPID()
	}

	return 0, derrors.New(
		derrors.CodeNotSupported,
		"FocusedApplicationPID not yet implemented on linux backend "+s.backend,
	)
}

// ApplicationNameByPID returns the name of the application with the given
// PID. It reads procfs, so it works on every backend.
func (s *SystemAdapter) ApplicationNameByPID(ctx context.Context, pid int) (string, error) {
	return linuxApplicationNameByPID(pid)
}

// ApplicationBundleIDByPID returns the desktop ID for pid, derived from
// procfs argv[0], so it works on every backend.
func (s *SystemAdapter) ApplicationBundleIDByPID(ctx context.Context, pid int) (string, error) {
	return linuxApplicationBundleIDByPID(pid)
}

// ScreenBounds returns the bounds of the active screen on Linux.
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

// ScreenScale returns the factor between the pixels ScreenBounds reports and
// the apparent size the user sees. X11 has one coordinate space in physical
// pixels and one desktop-wide Xft.dpi factor. Wayland bounds are logical, so
// the factor is 1.
func (s *SystemAdapter) ScreenScale(ctx context.Context, bounds image.Rectangle) (float64, error) {
	if s.backend == backendX11 {
		return x11DisplayScale()
	}

	if s.waylandUsesWlrClientStack() || s.backend == backendWaylandGNOME {
		return 1, nil
	}

	return 0, derrors.New(
		derrors.CodeNotSupported,
		"ScreenScale not yet implemented on linux backend "+s.backend,
	)
}

// Screens returns every connected screen on Linux, from XRandR on X11 and
// from wl_output globals on the wlroots client stack.
func (s *SystemAdapter) Screens(ctx context.Context) ([]ports.Screen, error) {
	if s.backend == backendX11 {
		return x11Screens()
	}

	if s.waylandUsesWlrClientStack() {
		return wlrootsScreens()
	}

	return nil, derrors.New(
		derrors.CodeNotSupported,
		"Screens not yet implemented on linux backend "+s.backend,
	)
}

// FocusedWindowBounds returns the global bounds of the focused window, used to
// constrain detection to its monitor. X11 reads _NET_ACTIVE_WINDOW geometry.
// Wayland exposes no client geometry, so the compositor answers (an IPC CLI on
// wlroots, the KWin script on KDE).
//
// No source at all is CodeNotSupported, so a caller knows it is falling back.
// found=false with a nil error means no bounds, usually an unfocused desktop.
func (s *SystemAdapter) FocusedWindowBounds(
	ctx context.Context,
) (image.Rectangle, bool, error) {
	if s.backend == backendX11 {
		return x11FocusedWindowBounds()
	}

	if s.waylandUsesWlrClientStack() {
		return waylandFocusedWindowBounds(s.backend)
	}

	return image.Rectangle{}, false, derrors.New(
		derrors.CodeNotSupported,
		"FocusedWindowBounds not yet implemented on linux backend "+s.backend,
	)
}

// MoveCursorToPoint moves the mouse cursor to point. With
// smooth_cursor.move_mouse_enabled and no bypass, the shared animator steps
// moveCursorDirect over time, and callers that need it settled pair this with
// WaitForCursorIdle. Otherwise it warps directly.
func (s *SystemAdapter) MoveCursorToPoint(
	ctx context.Context,
	point image.Point,
	bypassSmooth bool,
) error {
	// An absolute move supersedes pending relative deltas.
	s.relativeAnimator.stop()

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

	// Stop before warping so a pending tween step cannot override the warp.
	// stop() drains the injection mutex, so no step is in flight on return.
	// Two concurrent callers are still last-writer-wins, as on macOS.
	s.cursorAnimator.stop()

	return s.moveCursorDirect(point)
}

// MoveCursorInstantly posts one cursor move and returns at once
// (ports.InstantCursorMover), stopping both animators first. On wlroots it is
// a relative delta against the client-side cache, because nothing can query
// the real pointer and an absolute warp would jump by the cache's staleness.
func (s *SystemAdapter) MoveCursorInstantly(ctx context.Context, point image.Point) error {
	err := ctx.Err()
	if err != nil {
		return err
	}

	s.relativeAnimator.stop()
	s.cursorAnimator.stop()

	if s.backend == backendWaylandWlroots {
		cached, err := waylandCursorPosition()
		if err != nil {
			return err
		}

		return wlrootsMoveCursorBy(point.Sub(cached))
	}

	return s.moveCursorDirect(point)
}

// MoveCursorBy applies a relative cursor move. With smooth cursor enabled it
// animates over smooth_cursor.relative_movement_duration. wlroots drains the
// delta through native relative motion, never reading the stale position
// cache, and X11 and KDE extend the absolute animator's endpoint like macOS.
//
// With smooth cursor disabled, wlroots posts one native delta and everything
// else reports handled == false so callers keep their absolute fallback.
func (s *SystemAdapter) MoveCursorBy(
	ctx context.Context,
	delta image.Point,
) (bool, error) {
	cfg := currentLinuxConfig()
	// On CGO-off builds the injector is the loud stub, but the animator drops
	// injection errors, so animating would silence it. Fall through instead.
	if cfg != nil && cfg.SmoothCursor.MoveMouseEnabled && nativeBackendsCompiledIn {
		if s.backend == backendWaylandWlroots {
			// A failed drain injection could not be surfaced, so route moves
			// through the direct path, which errors loudly, until a success
			// proves recovery and re-arms animation.
			if s.relativeAnimator.injectionFailurePending() {
				err := wlrootsMoveCursorBy(delta)
				if err == nil {
					s.relativeAnimator.clearInjectionFailure()
				}

				return true, err
			}

			// Finish any absolute glide first so the delta drain composes from
			// where that animation was headed, not against its remaining steps.
			s.cursorAnimator.settle()
			s.relativeAnimator.addDelta(
				delta,
				cfg.SmoothCursor.Steps,
				cfg.SmoothCursor.RelativeMovementDuration,
			)

			return true, nil
		}

		if s.backend == backendX11 || backendCapturesViaPortal(s.backend) {
			bounds, err := s.ScreenBounds(ctx)
			if err == nil {
				s.cursorAnimator.animateRelativeBy(
					delta,
					func(point image.Point) image.Point {
						return image.Point{
							X: geometry.ClampInt(
								point.X,
								bounds.Min.X,
								max(bounds.Max.X-1, bounds.Min.X),
							),
							Y: geometry.ClampInt(
								point.Y,
								bounds.Min.Y,
								max(bounds.Max.Y-1, bounds.Min.Y),
							),
						}
					},
					cfg.SmoothCursor.Steps,
					cfg.SmoothCursor.RelativeMovementDuration,
				)

				return true, nil
			}
			// Without bounds, fall through to the caller's clamping fallback.
		}
	}

	if s.backend == backendWaylandWlroots {
		return true, wlrootsMoveCursorBy(delta)
	}

	return false, nil
}

// WaitForCursorIdle blocks until both the absolute glide and the wlroots
// relative drain settle, or ctx is canceled.
func (s *SystemAdapter) WaitForCursorIdle(ctx context.Context) error {
	err := s.cursorAnimator.wait(ctx)
	if err != nil {
		return err
	}

	return s.relativeAnimator.wait(ctx)
}

// SettleCursor finishes any in-flight cursor animation immediately, jumping to
// its endpoint. Actions call it before reading the cursor, so one fired
// mid-animation acts where the user aimed without waiting it out.
func (s *SystemAdapter) SettleCursor(ctx context.Context) error {
	s.cursorAnimator.settle()
	s.relativeAnimator.settle()

	return nil
}

// CursorPosition returns the current cursor position on Linux.
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

// SyncCursorPosition refreshes Wayland's client-side cursor cache. Wayland has
// no global pointer query, so it briefly maps transparent layer-shell surfaces
// and captures wl_pointer.enter coordinates.
func (s *SystemAdapter) SyncCursorPosition(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Mutter has no layer shell, but an override-redirect Xwayland window
	// receives the pointer's entry in global coordinates.
	if s.backend == backendWaylandGNOME {
		return xwaylandRefreshCursorPosition(ctx)
	}

	if s.waylandUsesWlrClientStack() {
		return waylandRefreshCursorPosition(ctx)
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

// IsSecureInputEnabled returns false on Linux. Secure input is a macOS-only
// concept.
func (s *SystemAdapter) IsSecureInputEnabled() bool {
	return false
}

// ShowSecureInputNotification is a no-op on Linux. Secure input is a
// macOS-only concept.
func (s *SystemAdapter) ShowSecureInputNotification() {}

// ShowAlert displays a message the user has to dismiss, through the session's
// notification daemon on every backend. See alertNotification for why it is a
// critical-urgency notification rather than a modal dialog.
func (s *SystemAdapter) ShowAlert(ctx context.Context, title, message string) error {
	return ShowAlert(ctx, title, message)
}

// ShowNotification displays a notification through the session's daemon,
// reporting CodeNotSupported when there is no bus or daemon.
func (s *SystemAdapter) ShowNotification(ctx context.Context, title, message string) error {
	return ShowNotification(ctx, title, message)
}

// CheckScreenCapturePermission reports whether the screen can be read right
// now, without prompting.
//
// X11 and wlroots have no gate and report true, the "no such gate" answer
// ports/system.go specifies. Portal-capture backends (KWin, cosmic-comp,
// Mutter) implement no screencopy, so capture goes through the ScreenCast
// portal, which is a consent gate. A build without native backends has no
// capture, and the capture's own refusal names CGO.
func (s *SystemAdapter) CheckScreenCapturePermission(_ context.Context) bool {
	if !backendCapturesViaPortal(s.backend) || !nativeBackendsCompiledIn {
		return true
	}

	return screenCastConsentHeld()
}

// RequestScreenCapturePermission establishes portal screen-sharing consent and
// reports what the user chose. Backends with no gate report granted.
//
// It runs the ScreenCast handshake, which shows the source picker only when
// there is no stored grant. It blocks, so the caller must not hold a lock
// across it. All prompting happens here, so no capture waits on a dialog.
func (s *SystemAdapter) RequestScreenCapturePermission(
	ctx context.Context,
) ports.ScreenCaptureConsent {
	if !backendCapturesViaPortal(s.backend) || !nativeBackendsCompiledIn {
		return ports.ScreenCaptureGranted
	}

	return requestScreenCastConsent(ctx)
}

// capabilityProbeTimeout bounds how long a single capability probe may take
// before it is treated as unavailable.
const capabilityProbeTimeout = 2 * time.Second

// probedCapability reports declared when probe succeeds, and a stub explaining
// why when it does not. Any failure downgrades, not just CodeNotSupported,
// since a compiled-in backend that cannot open its display is not supported.
func (s *SystemAdapter) probedCapability(
	feature string,
	declared ports.FeatureCapability,
	probe func() error,
) ports.FeatureCapability {
	completed, err := s.probes.run(feature, capabilityProbeTimeout, probe)

	switch {
	case !completed:
		return ports.FeatureCapability{
			Status: ports.FeatureStatusStub,
			Detail: feature + " is unavailable: the " + s.backendLabel() +
				" backend did not respond within " + capabilityProbeTimeout.String() +
				"; the display server may be wedged",
		}
	case err != nil:
		return ports.FeatureCapability{
			Status: ports.FeatureStatusStub,
			Detail: s.unavailableDetail(feature, err),
		}
	default:
		return declared
	}
}

// notificationFeature names the notification probe's slot and opens its detail.
const notificationFeature = "desktop notifications"

// notificationCapability live-probes whether a notification daemon is
// reachable right now.
//
// It shares probedCapability's slots and budget but not its wording, which
// blames the display-server backend. The budget is the full one because the
// first probe also pays for the session-bus connect.
func (s *SystemAdapter) notificationCapability(
	declared ports.FeatureCapability,
) ports.FeatureCapability {
	completed, err := s.probes.run(
		notificationFeature,
		capabilityProbeTimeout,
		func() error {
			ctx, cancel := context.WithTimeout(context.Background(), capabilityProbeTimeout)
			defer cancel()

			return sessionNotifier.daemonReachable(ctx)
		},
	)

	switch {
	case !completed:
		return ports.FeatureCapability{
			Status: ports.FeatureStatusStub,
			Detail: notificationFeature + " could not be confirmed: the session bus did not " +
				"answer within " + capabilityProbeTimeout.String(),
		}
	case err != nil:
		return ports.FeatureCapability{
			Status: ports.FeatureStatusStub,
			Detail: notificationFeature + " are unavailable: " + userFacingReason(err),
		}
	default:
		return declared
	}
}

// userFacingReason unwraps a domain error to its user sentence, without the
// "[CODE] ..." prefix, for a detail `neru doctor` prints verbatim.
func userFacingReason(err error) string {
	if domainErr, ok := errors.AsType[*derrors.Error](err); ok {
		return domainErr.Message()
	}

	return err.Error()
}

// unavailableDetail explains why a probed capability is unavailable, in terms
// the user can act on.
func (s *SystemAdapter) unavailableDetail(feature string, cause error) string {
	// An unfocused desktop still downgrades the entry, so doctor agrees with
	// callers, but "unavailable" would send the user to install something when
	// focusing a window is the fix. It comes first because the sentinel proves
	// a native backend answered, which makes every explanation below wrong.
	if errors.Is(cause, errNoFocusedWindow) {
		return feature + " found no focused window on linux backend " + s.backendLabel() +
			": the query works and answers as soon as a window takes focus"
	}

	// A focused window with no _NET_WM_PID is not broken either, since EWMH
	// does not require it. A separate sentinel keeps the advice accurate.
	if errors.Is(cause, errNoWindowPID) {
		return feature + " found no _NET_WM_PID on the focused window on linux backend " +
			s.backendLabel() + ": the query works and answers for a window that publishes one"
	}

	if !nativeBackendsCompiledIn {
		return feature + " is unavailable: this binary was built without CGO, so the X11 " +
			"and wlroots client stacks are absent; use a CGO-enabled build"
	}

	if s.backend == "" {
		return feature + " is unavailable: no display backend detected; start a session " +
			"under X11, or a Wayland compositor with wlr-foreign-toplevel/layer-shell support"
	}

	if s.backend == backendX11 || s.waylandUsesWlrClientStack() {
		// A live failure, not a gap. The underlying reason is the actionable part.
		return feature + " is unavailable on linux backend " + s.backend + ": " + cause.Error()
	}

	return feature + " is not implemented on linux backend " + s.backend +
		"; supported backends are x11, wayland-wlroots, wayland-kde, wayland-cosmic and wayland-gnome"
}

// backendLabel names the backend for diagnostics, including the undetected case.
func (s *SystemAdapter) backendLabel() string {
	if s.backend == "" {
		return "undetected"
	}

	return s.backend
}

// capabilityProbes serializes capability probing per feature. A timed-out
// probe stays blocked in an uncancellable native call, so one slot per feature
// caps outstanding probes at one each against a wedged display server.
type capabilityProbes struct {
	mu    sync.Mutex
	state map[string]*probeRun
}

// probeRun tracks a single feature's probe slot and its most recent result.
type probeRun struct {
	inFlight bool
	haveLast bool
	lastErr  error
}

func newCapabilityProbes() *capabilityProbes {
	return &capabilityProbes{state: make(map[string]*probeRun)}
}

// run probes feature, returning whether a result is available and what it was.
// While an earlier probe is stuck it starts none and reports the last result,
// so a slow backend keeps its last known state instead of "timed out".
func (p *capabilityProbes) run(
	feature string,
	timeout time.Duration,
	probe func() error,
) (bool, error) {
	p.mu.Lock()

	run, ok := p.state[feature]
	if !ok {
		run = &probeRun{}
		p.state[feature] = run
	}

	if run.inFlight {
		haveLast, lastErr := run.haveLast, run.lastErr
		p.mu.Unlock()

		return haveLast, lastErr
	}

	run.inFlight = true
	p.mu.Unlock()

	done := make(chan error, 1)

	go func() {
		err := probe()

		p.mu.Lock()
		run.inFlight = false
		run.haveLast = true
		run.lastErr = err
		p.mu.Unlock()

		done <- err
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case err := <-done:
		return true, err
	case <-timer.C:
		return false, nil
	}
}

// moveCursorDirect performs one instant cursor warp through the backend's
// injector. It is the sink for direct moves and each animator step.
func (s *SystemAdapter) moveCursorDirect(point image.Point) error {
	if s.backend == backendX11 {
		return x11MoveCursorToPoint(point)
	}

	if s.waylandUsesWlrClientStack() {
		// The dispatcher sends KDE (no virtual pointer) to libei.
		return waylandMoveCursorToPoint(point)
	}

	return derrors.New(derrors.CodeNotSupported, "MoveCursorToPoint not yet implemented on linux")
}

// currentCursorPosition seeds the animator, collapsing an error to a zero
// point. A bad read only skews the glide, never the landing point.
func (s *SystemAdapter) currentCursorPosition() image.Point {
	point, err := s.CursorPosition(context.Background())
	if err != nil {
		return image.Point{}
	}

	return point
}

// waylandUsesWlrClientStack is true when the session is served by the shared
// wlroots client (xdg-output, the cursor cache, whichever input protocols the
// compositor offers). Mutter implements only xdg-output, and GNOME's other
// capabilities come from portals or Xwayland.
func (s *SystemAdapter) waylandUsesWlrClientStack() bool {
	return backendUsesWlrClientStack(s.backend)
}

// backendUsesWlrClientStack answers the same for a bare backend label, which
// screen capture routes on. A new wlr-family backend is added here only.
func backendUsesWlrClientStack(backend string) bool {
	return backend == backendWaylandWlroots || backend == backendWaylandKDE ||
		backend == backendWaylandCOSMIC || backend == backendWaylandGNOME
}

// backendCapturesViaPortal names the backends with no screencopy protocol
// (KWin, cosmic-comp, Mutter), which capture through the consent-gated
// ScreenCast portal. Consent checks and the smooth-cursor animator route on it.
func backendCapturesViaPortal(backend string) bool {
	return backend == backendWaylandKDE || backend == backendWaylandCOSMIC ||
		backend == backendWaylandGNOME
}

// Ensure SystemAdapter implements ports.SystemPort.
var _ ports.SystemPort = (*SystemAdapter)(nil)

// Callers reach these optional extensions by type assertion, so signature
// drift would silently downgrade Linux instead of failing to compile.
var (
	_ ports.RelativeCursorMover = (*SystemAdapter)(nil)
	_ ports.CursorSynchronizer  = (*SystemAdapter)(nil)
	_ ports.CursorSettler       = (*SystemAdapter)(nil)
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

// darkModePortalTimeout caps the busctl call, normally sub-millisecond, so a
// wedged portal cannot block `neru doctor`.
const darkModePortalTimeout = 250 * time.Millisecond

// portalBusctlMinFields is the minimum token count in a successful busctl
// Settings.Read response ("v v u N").
const portalBusctlMinFields = 4

// darkModePreference returns the active freedesktop color-scheme preference,
// trying the xdg-desktop-portal Settings.Read interface first, then
// kdeglobals for Plasma installs without xdg-desktop-portal-kde. ok=false
// means no source could be queried, which is "unknown", not "light".
func darkModePreference() (int, darkModeSource, bool) {
	if value, ok := readPortalColorScheme(); ok {
		return value, darkModeSourcePortal, true
	}

	if value, ok := readKDEColorScheme(); ok {
		return value, darkModeSourceKDEGlobals, true
	}

	return -1, "", false
}

// readPortalColorScheme queries the xdg-desktop-portal Settings interface,
// which busctl prints as e.g. "v v u 1". Never pass --quiet, which also
// suppresses the return value.
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
	// Fewer tokens means busctl printed something unrecognized.
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

// readKDEColorScheme infers a color scheme from [General] ColorScheme in
// kdeglobals (or the kdedefaults variant). A name containing "dark" (any case,
// such as BreezeDark) is dark, anything else light. ok=false when neither file
// has the key.
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

// scanINIValue is a minimal INI section/key reader for kdeglobals, avoiding an
// INI parser dependency. It stays generic so parsing and dark-mode policy are
// tested apart.
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

// darkModeCapability describes the current dark-mode state for `neru doctor`.
// When ok=false it is a stub with a fix-it hint, since the user likely has a
// preference we cannot see.
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

// SystemAdapter posts single moves without waiting for the held-key glide.
var _ ports.InstantCursorMover = (*SystemAdapter)(nil)

// gnomeAppWatcherCapability reports the watcher as the extension's state: live
// when the extension answers, a stub naming the fix when it does not.
func gnomeAppWatcherCapability() ports.FeatureCapability {
	bridge := gnomeshell.Shared(nil)
	bridge.EnsureStarted()

	_, _, err := bridge.Focused()
	if err != nil {
		return ports.FeatureCapability{
			Status: ports.FeatureStatusStub,
			Detail: "focused-app watcher needs the Neru GNOME Shell extension: " + err.Error() +
				"; enable " + gnomeshell.UUID + " and log out and back in",
		}
	}

	return ports.FeatureCapability{
		Status: ports.FeatureStatusSupported,
		Detail: "focused-app change detection keyed on the app_id the Neru GNOME Shell extension reports, event-driven",
	}
}
