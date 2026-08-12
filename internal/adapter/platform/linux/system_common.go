//go:build linux

//nolint:godox // TODO comments are intentional contributor guidance for unimplemented stubs.
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

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/geometry"
	"github.com/y3owk1n/neru/internal/ports"
)

const (
	backendX11            = "x11"
	backendWaylandWlroots = "wayland-wlroots"
	backendWaylandKDE     = "wayland-kde"
	// backendUnknown mirrors platform.LinuxBackend.String() for BackendUnknown.
	// This package cannot import platform (the factory there imports this one),
	// so the label is duplicated rather than referenced.
	backendUnknown = "unknown"
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
	adapter := &SystemAdapter{backend: backend, probes: newCapabilityProbes()}
	adapter.cursorAnimator = newSmoothCursorAnimator(
		adapter.currentCursorPosition,
		adapter.moveCursorDirect,
	)
	// Only the wlroots backend drives the delta-drain animator (it is the one
	// backend with native relative motion); constructing it unconditionally
	// keeps every method a safe no-op elsewhere.
	adapter.relativeAnimator = newRelativeCursorAnimator(wlrootsMoveCursorBy)

	warmFocusedWindowSource(backend)

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

	// Notifications are live-probed for the same reason dark mode is: the code
	// path exists on every backend, but whether the user will see anything
	// depends on a notification daemon being installed, which is a session fact
	// rather than a build one.
	capabilities.Notifications = s.notificationCapability(capabilities.Notifications)

	// Screen, cursor and process support is what this binary and session can
	// actually reach, not what the build target intends — the static preset is
	// wrong for CGO_ENABLED=0, for compositors without the wlroots stack, and
	// for unopenable displays. Each is probed through the same read-only call
	// the capability describes, so doctor cannot disagree with what a caller
	// observes. Only doctor and the info response reach this, so the reads are
	// cheap.
	capabilities.Process = s.probedCapability("focused-app inspection", capabilities.Process,
		func() error {
			_, err := s.FocusedApplicationPID(context.Background())

			return err
		})
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

	return capabilities
}

// xdgDir resolves an XDG base directory per the Base Directory spec: the
// environment variable wins when set to an absolute path (relative values
// must be ignored per the spec), otherwise the given default under $HOME.
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
// Wayland has no protocol exposing another client's on-screen geometry, so the
// answer comes from the compositor: an IPC CLI on the wlroots family, the KWin
// geometry script on KDE.
//
// A compositor with no source at all says so with CodeNotSupported, because a
// caller that falls back to the active screen should know it is falling back.
// found=false with a nil error is the ordinary "no bounds available" answer —
// most often an unfocused desktop.
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
	// An absolute move supersedes any pending relative deltas: the cursor is
	// about to be placed somewhere explicit, so flushing the drain would only
	// fight the warp. Discard it before either branch below.
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

	// Stop before warping: the animation worker must be canceled first so a
	// pending tween step cannot override the direct warp. stop() drains the
	// animator's injection mutex, so on return no animation step is in flight
	// and none will start — this warp lands last, not racing a stale step back
	// to an intermediate point. (Which of two distinct concurrent callers wins
	// is still last-writer-wins, as on macOS.)
	s.cursorAnimator.stop()

	return s.moveCursorDirect(point)
}

// MoveCursorBy applies a relative cursor move. With smooth cursor enabled
// (smooth_cursor.move_mouse_enabled) the move animates over the fixed
// per-move duration smooth_cursor.relative_movement_duration:
//
//   - wlroots drains the delta in integer chunks through native relative
//     motion — never reading the client position cache, whose staleness is
//     why this backend applies deltas natively in the first place;
//   - X11 and KDE extend the absolute animator's pending endpoint, exactly
//     like macOS (KDE's fallback path was already cache-based absolute, so
//     nothing is lost).
//
// With smooth cursor disabled, behavior is unchanged: wlroots posts one
// native delta, everything else reports handled == false so callers keep
// their absolute-position fallback.
func (s *SystemAdapter) MoveCursorBy(
	ctx context.Context,
	delta image.Point,
) (bool, error) {
	cfg := currentLinuxConfig()
	// nativeBackendsCompiledIn gates the animated paths: on CGO-off builds the
	// drain's injector is the loud CodeNotSupported stub, but the animator
	// drops injection errors by design — animating would launder the stub into
	// a silent no-op. Falling through keeps the error surfaced to the caller.
	if cfg != nil && cfg.SmoothCursor.MoveMouseEnabled && nativeBackendsCompiledIn {
		if s.backend == backendWaylandWlroots {
			// A failed injection during a drain could not be surfaced (its
			// move had already reported handled). Until recovery is proven,
			// route every move through the direct native path: it returns the
			// backend error loudly, and only a success — the proof the
			// backend recovered — re-arms animation.
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

		if s.backend == backendX11 || s.backend == backendWaylandKDE {
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
			// Without bounds there is nothing to clamp against; fall through to
			// the caller's fallback, which clamps at the service layer.
		}
	}

	if s.backend == backendWaylandWlroots {
		return true, wlrootsMoveCursorBy(delta)
	}

	return false, nil
}

// WaitForCursorIdle blocks until any in-flight smooth cursor animation settles,
// or ctx is canceled. It returns immediately when no animation is active — the
// common case on the direct (non-smooth) move path. Both animators are waited
// on: the absolute glide and the wlroots relative-delta drain.
func (s *SystemAdapter) WaitForCursorIdle(ctx context.Context) error {
	err := s.cursorAnimator.wait(ctx)
	if err != nil {
		return err
	}

	return s.relativeAnimator.wait(ctx)
}

// SettleCursor finishes any in-flight cursor animation immediately: the
// absolute animator warps straight to the endpoint it was animating toward,
// and the relative drain flushes its remaining delta in one native motion.
// Action paths call this before resolving their target point from the
// cursor, so an action firing mid-animation acts at the point the user aimed
// for — without paying the animation's remaining duration in latency.
func (s *SystemAdapter) SettleCursor(ctx context.Context) error {
	s.cursorAnimator.settle()
	s.relativeAnimator.settle()

	return nil
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

// IsSecureInputEnabled returns false on Linux — secure input is a macOS-only concept.
func (s *SystemAdapter) IsSecureInputEnabled() bool {
	return false
}

// ShowSecureInputNotification is a no-op on Linux — secure input is a macOS-only concept.
func (s *SystemAdapter) ShowSecureInputNotification() {}

// ShowAlert displays a message the user has to dismiss, through the session's
// freedesktop notification daemon. Delivery does not depend on the display
// server, so every Linux backend takes the same path; see alertNotification
// for why this is a critical-urgency notification rather than a modal dialog.
func (s *SystemAdapter) ShowAlert(ctx context.Context, title, message string) error {
	return ShowAlert(ctx, title, message)
}

// ShowNotification displays a lightweight notification through the session's
// freedesktop notification daemon, reporting CodeNotSupported when the session
// has no bus or no daemon to show it rather than dropping the message.
func (s *SystemAdapter) ShowNotification(ctx context.Context, title, message string) error {
	return ShowNotification(ctx, title, message)
}

// CheckScreenCapturePermission reports whether the screen can be read right
// now, without prompting.
//
// Two of the three Linux backends have no gate at all and report true, which is
// the "this platform has no such gate" answer ports/system.go specifies rather
// than a silent no-op: X11 reads the root window back and the wlroots family
// implements wlr-screencopy, and a client already trusted with the session
// needs no further permission for either.
//
// KDE Plasma is the exception and the reason this method stopped answering
// unconditionally. KWin implements no screencopy protocol, so capture goes
// through xdg-desktop-portal's ScreenCast session — which *is* a consent gate,
// and a preflight that reported it open regardless would leave the one Linux
// backend with a permission the only one whose permission was never checked.
//
// A build with no native backends compiled in has no gate either, because it
// has no capture: the refusal belongs to the capture, which names CGO, rather
// than to a consent prompt that could not help.
func (s *SystemAdapter) CheckScreenCapturePermission(_ context.Context) bool {
	if s.backend != backendWaylandKDE || !nativeBackendsCompiledIn {
		return true
	}

	return screenCastConsentHeld()
}

// RequestScreenCapturePermission establishes KDE's screen-sharing consent and
// reports what the user chose; on every other Linux backend it reports granted
// without showing anything, because there is nothing to ask for.
//
// On KDE it runs the ScreenCast handshake, which shows the portal's source
// picker when there is no stored grant to restore and shows nothing when there
// is. It blocks — the caller is contractually required not to hold a lock
// across it — and it is where the whole prompt budget is spent, so that no
// capture ever waits on a dialog.
func (s *SystemAdapter) RequestScreenCapturePermission(
	ctx context.Context,
) ports.ScreenCaptureConsent {
	if s.backend != backendWaylandKDE || !nativeBackendsCompiledIn {
		return ports.ScreenCaptureGranted
	}

	return requestScreenCastConsent(ctx)
}

// capabilityProbeTimeout bounds how long a single capability probe may take
// before it is treated as unavailable.
const capabilityProbeTimeout = 2 * time.Second

// probedCapability reports declared when probe succeeds, and a stub explaining
// why when it does not.
//
// Any probe failure downgrades the capability, not just CodeNotSupported. The
// question `neru doctor` answers is "does this work right now?", and a backend
// that is compiled in but cannot open its display fails with CodeActionFailed —
// reporting that as "supported" would be the same lie this probing exists to
// remove. The detail distinguishes the cases so the user knows whether to
// install something, start a session, or look at a broken display server.
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

// notificationFeature names the notification probe's slot in capabilityProbes,
// and opens the detail it reports.
const notificationFeature = "desktop notifications"

// notificationCapability live-probes whether a notification daemon is
// reachable right now. The question a user runs `neru doctor` to answer is
// "will I see notifications?", and on Linux that is answered by the session
// rather than by this code: every backend can send one, and a session with no
// daemon shows none. Reporting the static "supported" there would be the same
// lie the empty ShowNotification body used to tell.
//
// It shares the probe slots and the budget with probedCapability but not its
// body: that one explains a failure in terms of the display-server backend,
// which decides nothing here — notifications are a session-bus service every
// backend reaches the same way. The budget has to be the full one because the
// first probe of a daemon's life pays for the session-bus connect as well as
// the question, and reporting "could not be confirmed" on a session where
// notifications work is the same dishonesty in the other direction.
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

// userFacingReason unwraps a domain error to the sentence written for the
// user, leaving the "[CODE] …" prefix out of a capability detail that `neru
// doctor` prints verbatim.
func userFacingReason(err error) string {
	var domainErr *derrors.Error
	if errors.As(err, &domainErr) {
		return domainErr.Message()
	}

	return err.Error()
}

// unavailableDetail explains why a probed capability is unavailable, in terms
// the user can act on.
func (s *SystemAdapter) unavailableDetail(feature string, cause error) string {
	// An unfocused desktop is not an unavailable capability. The probe still
	// downgrades the entry — FocusedApplicationPID refuses right now, and the
	// matrix `neru doctor` prints has to agree with what a caller observes —
	// but describing that as "unavailable" sends the user looking for a portal
	// to install or a session to restart when focusing any window is the whole
	// fix. This branch comes first because reaching this sentinel at all proves
	// a native backend answered, which makes every explanation below wrong.
	if errors.Is(cause, errNoFocusedWindow) {
		return feature + " found no focused window on linux backend " + s.backendLabel() +
			": the query works and answers as soon as a window takes focus"
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
		// The backend is implemented, so this is a live failure rather than a
		// gap — carry the underlying reason, which is the actionable part.
		return feature + " is unavailable on linux backend " + s.backend + ": " + cause.Error()
	}

	return feature + " is not implemented on linux backend " + s.backend +
		"; supported backends are x11, wayland-wlroots and wayland-kde"
}

// backendLabel names the backend for diagnostics, including the undetected case.
func (s *SystemAdapter) backendLabel() string {
	if s.backend == "" {
		return "undetected"
	}

	return s.backend
}

// capabilityProbes serializes capability probing per feature.
//
// A probe that times out leaves its goroutine blocked in a native call that
// cannot be canceled. Without this, every subsequent doctor, status or health
// request against a wedged display server would start three more, so goroutines
// and native handles would grow without bound for as long as the backend stayed
// stuck. Holding one slot per feature caps the outstanding probes at exactly
// one each, however often capabilities are requested.
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
//
// When a probe from an earlier request is still stuck, run does not start
// another: it reports the previous result if there is one, so a transiently
// slow backend keeps answering with its last known state instead of degrading
// every capability to "timed out".
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
	return backendUsesWlrClientStack(s.backend)
}

// backendUsesWlrClientStack answers the same question for a backend label with
// no adapter in hand — screen capture routes on the label alone. One spelling
// so a future wlr-family backend is added in one place.
func backendUsesWlrClientStack(backend string) bool {
	return backend == backendWaylandWlroots || backend == backendWaylandKDE
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
