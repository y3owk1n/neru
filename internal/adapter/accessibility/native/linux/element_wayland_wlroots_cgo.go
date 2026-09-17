//go:build linux && cgo

package linux

import (
	"image"
	"os"
	"sync"

	eventtaplinux "github.com/y3owk1n/neru/internal/adapter/eventtap/linux"
	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/adapter/platform/mousestate"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// globalWlrootsPointerState records the modifiers each held button was pressed
// with, so that the matching release can undo exactly those modifiers.
var globalWlrootsPointerState mousestate.Tracker

func wlrootsFocusedApplicationIdentity() (string, int) {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return "", 0
	}

	// The wlr-foreign-toplevel-management protocol (wlroots + KWin/KDE) reports
	// the focused toplevel's app_id but not its PID, so the PID is 0. app_id is
	// the identifier used for per-app configuration lookups; when a caller needs
	// a real PID it falls back to the XWayland path if DISPLAY is set.
	appID, ok := linux.FocusedAppID(platform.DetectLinuxBackend().String())
	if ok && appID != "" {
		return appID, 0
	}

	// No focused app_id available (nothing is focused yet, or the GNOME
	// extension is not running). Fall through to the XWayland fallback.
	return "", 0
}

func wlrootsApplicationBundleIdentifier(pid int) string {
	_ = pid

	return ""
}

func wlrootsMoveMouseToPoint(point image.Point) error {
	return linux.WaylandMoveCursorToPoint(point)
}

func wlrootsCurrentCursorPosition() image.Point {
	pos, err := linux.WaylandCursorPosition()
	if err != nil {
		return image.Point{}
	}

	return pos
}

func wlrootsLeftClickAtPoint(
	point image.Point,
	restoreCursor bool,
	modifiers action.Modifiers,
) error {
	return wlrootsClickButtonAtPoint(point, restoreCursor, modifiers, linux.WlrBtnLeft)
}

func wlrootsRightClickAtPoint(
	point image.Point,
	restoreCursor bool,
	modifiers action.Modifiers,
) error {
	return wlrootsClickButtonAtPoint(point, restoreCursor, modifiers, linux.WlrBtnRight)
}

func wlrootsMiddleClickAtPoint(
	point image.Point,
	restoreCursor bool,
	modifiers action.Modifiers,
) error {
	return wlrootsClickButtonAtPoint(point, restoreCursor, modifiers, linux.WlrBtnMiddle)
}

// wlrootsButton maps a domain mouse button to its wlroots button code.
func wlrootsButton(button action.MouseButton) int {
	switch button {
	case action.ButtonRight:
		return linux.WlrBtnRight
	case action.ButtonMiddle:
		return linux.WlrBtnMiddle
	case action.ButtonLeft:
		fallthrough
	default:
		return linux.WlrBtnLeft
	}
}

// physicalLift is the shared lifetime of one lift of the user's held chord.
// Every pointer action that can overlap another takes a hold: a scroll from
// its first event to its last, a drag from press to release. The chord goes
// back on only when the last hold is gone, whichever action that turns out
// to be. The proxy itself is not nestable, since a second lift finds nothing
// left to lift and any restore puts everything back, so the counting lives
// here.
type physicalLift struct {
	mu    sync.Mutex
	holds int

	// lifted is whether the proxy has the chord up on Neru's account, so a
	// restore with nothing to put back skips the compositor barrier.
	lifted bool

	// liftHeld and restoreHeld are the proxy calls, fields so a test of who
	// restores can run without one.
	liftHeld    func() (bool, error)
	restoreHeld func()
}

var globalPhysicalLift = physicalLift{
	liftHeld:    eventtaplinux.LiftHeldModifiers,
	restoreHeld: restoreLiftedModifiersAfterSync,
}

// restoreLiftedModifiersAfterSync presses the lifted modifiers again once the
// compositor has processed the action. WaylandSyncModifiers is the barrier.
func restoreLiftedModifiersAfterSync() {
	if !linux.WaylandSyncModifiers(modifierSyncTimeout) {
		waitForScrollDelivery()
	}

	_ = eventtaplinux.RestoreLiftedModifiers()
}

// lift releases the chord on the proxy. A fresh lift waits the fixed period
// the uinput side owes; one that found the chord already up owes nothing.
func (l *physicalLift) lift() {
	lifted, err := l.liftHeld()
	if err != nil || !lifted {
		return
	}

	l.mu.Lock()
	l.lifted = true
	l.mu.Unlock()

	waitForScrollDelivery()
}

// acquire lifts the chord and counts a hold for the caller.
func (l *physicalLift) acquire() {
	l.mu.Lock()
	l.holds++
	l.mu.Unlock()

	l.lift()
}

// release drops one hold and puts the chord back when no hold and no held
// button remains. A button counts on its own rather than as a hold because a
// release that fails keeps the button recorded for the idle cleanup to retry,
// and a retry must not drop a hold twice.
func (l *physicalLift) release() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.holds > 0 {
		l.holds--
	}

	if l.holds > 0 || len(globalWlrootsPointerState.HeldButtons()) > 0 {
		return
	}

	l.restoreLocked()
}

// restoreUnlessHeld puts the chord back after a drag ends, unless a button
// other than the one just released is still down or a scroll still holds the
// lift. The drag's press took no hold, so none is dropped.
func (l *physicalLift) restoreUnlessHeld(button action.MouseButton) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.holds > 0 {
		return
	}

	for _, held := range globalWlrootsPointerState.HeldButtons() {
		if held != button {
			return
		}
	}

	l.restoreLocked()
}

// restoreLocked puts back the lifted modifiers after the action, or the
// release that ends a drag. Nothing lifted means nothing to wait for.
func (l *physicalLift) restoreLocked() {
	if !l.lifted {
		return
	}

	l.lifted = false

	l.restoreHeld()
}

// liftPhysicalModifiers releases the modifiers the user's hand is on for the
// length of a pointer action, so the action carries only the set it names.
// This is the x11ClickButtonAtPoint rule applied to the one keyboard the
// compositor reads here, the evdev proxy's (footnote 7 of
// docs/CROSS_PLATFORM.md). The releases go out on uinput and the action on the
// Wayland socket, and nothing orders the two, so a lift waits the fixed period
// the uinput side always waits (waitForScrollDelivery). The returned restore
// presses the lifted modifiers again once every overlapping action is done.
func liftPhysicalModifiers() func() {
	globalPhysicalLift.acquire()

	return globalPhysicalLift.release
}

func wlrootsMouseDownAtPoint(
	point image.Point,
	button action.MouseButton,
	modifiers action.Modifiers,
) error {
	// Lifted for the whole drag, as on X11: the release that ends the last
	// held button restores, so the press takes no hold of its own. A press
	// that fails restores here, since no release is coming for it.
	globalPhysicalLift.lift()

	err := wlrootsPressModifiers(modifiers)
	if err != nil {
		restoreUnlessOtherHeld(button)

		return err
	}

	err = linux.WaylandButtonEvent(point, wlrootsButton(button), true)
	if err != nil {
		_ = wlrootsReleaseModifiers(modifiers)

		restoreUnlessOtherHeld(button)

		return err
	}

	globalWlrootsPointerState.SetDown(button, point, modifiers)

	return nil
}

// restoreAfterRelease puts the lifted modifiers back once the button just
// released was the last one held, so a release of an unrelated button does
// not re-modify a drag still in progress.
func restoreAfterRelease(button action.MouseButton) {
	globalWlrootsPointerState.Clear(button)
	restoreUnlessOtherHeld(button)
}

// restoreUnlessOtherHeld puts the lifted modifiers back unless a button other
// than this one is still held. A release that failed goes through here with
// its button still recorded, so the idle cleanup can retry it: the modifiers
// come back regardless, which is the documented bias, since the opposite
// drops a modifier the user is still holding.
func restoreUnlessOtherHeld(button action.MouseButton) {
	globalPhysicalLift.restoreUnlessHeld(button)
}

func wlrootsMouseUpAtPoint(
	point image.Point,
	button action.MouseButton,
	modifiers action.Modifiers,
) error {
	heldModifiers, hadMouseDown := globalWlrootsPointerState.DownModifiers(button)
	if hadMouseDown {
		return wlrootsReleaseHeldButton(point, button, heldModifiers)
	}

	// A release with no press behind it is its own pointer action, so it
	// lifts and restores around itself the way a click does.
	restore := liftPhysicalModifiers()
	defer restore()

	err := wlrootsPressModifiers(modifiers)
	if err != nil {
		return err
	}

	defer func() {
		_ = wlrootsReleaseModifiers(modifiers)
	}()

	return linux.WaylandButtonEvent(point, wlrootsButton(button), false)
}

// wlrootsReleaseHeldButton ends a press this process recorded, letting go of
// the modifiers it pressed. A failed release keeps the button recorded for the
// idle cleanup to retry, naming only the modifiers whose release also failed:
// the rest are up already, and a second release of a modifier Neru no longer
// holds lets go of the user's own.
func wlrootsReleaseHeldButton(
	point image.Point,
	button action.MouseButton,
	modifiers action.Modifiers,
) error {
	err := linux.WaylandButtonEvent(point, wlrootsButton(button), false)
	if err != nil {
		remaining, _ := releaseWaylandModifiersRemaining(modifiers)

		if position, ok := globalWlrootsPointerState.DownPosition(button); ok {
			globalWlrootsPointerState.SetDown(button, position, remaining)
		}

		restoreUnlessOtherHeld(button)

		return err
	}

	_ = wlrootsReleaseModifiers(modifiers)

	restoreAfterRelease(button)

	return nil
}

func wlrootsClickButtonAtPoint(
	point image.Point,
	restoreCursor bool,
	modifiers action.Modifiers,
	button int,
) error {
	original := wlrootsCurrentCursorPosition()

	restore := liftPhysicalModifiers()
	defer restore()

	err := wlrootsPressModifiers(modifiers)
	if err != nil {
		return err
	}

	defer func() {
		_ = wlrootsReleaseModifiers(modifiers)
	}()

	err = linux.WaylandClick(point, button)
	if err != nil {
		return err
	}

	if restoreCursor {
		_ = linux.WaylandMoveCursorToPoint(original)
	}

	return nil
}

// wlrootsPressModifiers presses the modifiers a pointer action presents. A
// failure unwinds only what went down, which is pressWaylandModifiers' job.
func wlrootsPressModifiers(modifiers action.Modifiers) error {
	_, err := pressWaylandModifiers(modifiers)

	return err
}

// wlrootsReleaseModifiers lets go of the modifiers a pointer action presented.
//
// It takes the set the matching press was given rather than the set it got
// down, because the two callers that hold across a boundary — a drag, and a
// release with no press behind it — have only the former to hand back.
func wlrootsReleaseModifiers(modifiers action.Modifiers) error {
	return releaseWaylandModifiers(modifiers)
}

func wlrootsMouseUp(button action.MouseButton) error {
	modifiers, hadMouseDown := globalWlrootsPointerState.DownModifiers(button)

	err := linux.WaylandButtonRelease(wlrootsButton(button))
	if err != nil {
		if hadMouseDown {
			restoreUnlessOtherHeld(button)
		}

		return err
	}

	if hadMouseDown {
		_ = wlrootsReleaseModifiers(modifiers)

		restoreAfterRelease(button)
	}

	return nil
}

// wlrootsScrollStep is the pixel value one virtual-pointer notch carries,
// the same figure the uinput path counts notches in, so the two backends
// travel the same distance from the same scroll service delta.
const (
	wlrootsScrollMaxEvents = 50
	wlrootsScrollStep      = scrollPixelsPerNotch // pixels per notch
)

// wlrootsScrollAtCursor emits the scroll on the wlroots virtual pointer, with
// modifiers held on the virtual keyboard (libei on KDE) for its duration.
//
// Both halves go out through the same seat, which is the whole reason a
// modified scroll is routed here rather than through the faster uinput batch.
// The release only lets go of what this call pressed, so a modifier the user is
// physically holding survives it.
func wlrootsScrollAtCursor(deltaX, deltaY int, modifiers action.Modifiers) error {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return derrors.New(
			derrors.CodeNotSupported,
			"WAYLAND_DISPLAY is not set; wlroots backend is unavailable",
		)
	}

	err := wlrootsPressModifiers(modifiers)
	if err != nil {
		return err
	}

	defer func() {
		_ = wlrootsReleaseModifiers(modifiers)
	}()

	if deltaY != 0 {
		err := wlrootsScrollAxis(0, deltaY)
		if err != nil {
			return err
		}
	}

	if deltaX != 0 {
		err := wlrootsScrollAxis(1, deltaX)
		if err != nil {
			return err
		}
	}

	return nil
}

// waylandScrollSession injects an animated scroll on Wayland as continuous axis
// events, holding any modifiers for the length of the animation.
//
// It uses the continuous axis rather than the discrete one the unanimated path
// sends, and that choice is the whole reason smooth scroll works here: an axis
// event with no discrete step count reaches the focused client as the fraction
// it carries, while a discrete one declares a wheel notch that the compositor
// may hold back until whole notches accumulate. Both wlroots (through
// zwlr_virtual_pointer) and KWin (through libei's pixel-precise scroll delta)
// pass the fraction through; linux.WaylandScrollContinuous picks between them.
type waylandScrollSession struct {
	modifiers action.Modifiers
	restore   func()
}

// waylandScrollBackendAvailable answers whether an animated scroll could inject
// here, without touching the compositor to find out.
func waylandScrollBackendAvailable() error {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return derrors.New(
			derrors.CodeNotSupported,
			"WAYLAND_DISPLAY is not set; wlroots backend is unavailable",
		)
	}

	return nil
}

func newWaylandScrollSession(modifiers action.Modifiers) (scrollSession, error) {
	err := waylandScrollBackendAvailable()
	if err != nil {
		return nil, err
	}

	restore := liftPhysicalModifiers()

	if modifiers != 0 {
		pressErr := wlrootsPressModifiers(modifiers)
		if pressErr != nil {
			restore()

			return nil, pressErr
		}
	}

	return &waylandScrollSession{modifiers: modifiers, restore: restore}, nil
}

// granularity is zero: a Wayland axis value is a distance, not a step count, so
// there is no unit to round to.
func (s *waylandScrollSession) granularity() float64 { return 0 }

func (s *waylandScrollSession) inject(deltaX, deltaY float64) error {
	if deltaY != 0 {
		// Wayland axis convention: positive = scroll down. Application
		// convention: positive delta = scroll up. Same negation the discrete
		// path applies in wlrootsScrollAxis.
		err := linux.WaylandScrollContinuous(0, -deltaY)
		if err != nil {
			return err
		}
	}

	if deltaX != 0 {
		return linux.WaylandScrollContinuous(1, deltaX)
	}

	return nil
}

// close releases only what this session pressed, so a modifier the user is
// physically holding survives the animation.
func (s *waylandScrollSession) close() {
	if s.modifiers != 0 {
		_ = wlrootsReleaseModifiers(s.modifiers)
	}

	s.restore()
}

// wlrootsScrollAxis sends Wayland axis events for one axis.
// Each event carries 1 notch (axis_discrete=±1, axis_value120=±120) to
// match what a physical mouse wheel produces — no toolkit clipping.
// Events are sent in batches of wlrootsScrollMaxEvents to avoid flooding
// the compositor socket.
//
// Wayland axis convention: positive = scroll down (axis 0) / right (axis 1).
// Application  convention: positive delta = scroll up (axis 0) / right (axis 1).
// Vertical axis sign is negated to convert between the two.
func wlrootsScrollAxis(axis int, delta int) error {
	totalNotches := scrollNotches(delta)

	step, disc := wlrootsScrollNotch(axis, delta, wlrootsScrollStep)

	deltas := make([]int, 0, wlrootsScrollMaxEvents)
	discretes := make([]int, 0, wlrootsScrollMaxEvents)
	remaining := totalNotches

	for remaining > 0 {
		deltas = append(deltas, step)
		discretes = append(discretes, disc)
		remaining--

		if len(deltas) >= wlrootsScrollMaxEvents || remaining == 0 {
			err := linux.WlrootsScrollBatch(axis, deltas, discretes)
			if err != nil {
				return err
			}

			deltas = deltas[:0]
			discretes = discretes[:0]
		}
	}

	return nil
}
