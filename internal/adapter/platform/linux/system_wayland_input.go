//go:build linux

package linux

import (
	"context"
	"image"
	"os"
	"time"

	"github.com/y3owk1n/neru/internal/derrors"
)

// evdev KEY_* codes for the libei modifier keyboard path.
const (
	keycodeLeftShift = 42  // KEY_LEFTSHIFT
	keycodeLeftCtrl  = 29  // KEY_LEFTCTRL
	keycodeLeftAlt   = 56  // KEY_LEFTALT
	keycodeLeftMeta  = 125 // KEY_LEFTMETA
)

// Modifier name constants used in key injection.
const (
	modNameShift = "shift"
	modNameCtrl  = "ctrl"
	modNameAlt   = "alt"
	modNameCmd   = "cmd"
)

// libeiModifierKeycodes maps Neru's modifier names to evdev keycodes for the
// libei keyboard path. KWin's RemoteDesktop portal commonly grants only a
// pointer device, so libeiKey may still report these as unsupported.
var libeiModifierKeycodes = map[string]int{
	modNameShift: keycodeLeftShift,
	modNameCtrl:  keycodeLeftCtrl,
	modNameAlt:   keycodeLeftAlt,
	modNameCmd:   keycodeLeftMeta,
}

// Release-retry backoff for the libei click path, spaced out because a device
// KWin paused between press and release needs time to resume.
const (
	libeiReleaseRetries = 5
	libeiReleaseBackoff = 100 * time.Millisecond
)

// libeiButtonRelease emits a button release, retrying with backoff while the
// device is paused. A lost release leaves the button held, turning the next
// move into a drag.
func libeiButtonRelease(button int) error {
	err := libeiButton(button, false)

	for attempt := 0; err != nil && attempt < libeiReleaseRetries; attempt++ {
		time.Sleep(libeiReleaseBackoff)

		err = libeiButton(button, false)
	}

	return err
}

// WarmWaylandInput pre-establishes the input backend at daemon startup. This
// file routes input to the wlroots virtual pointer and keyboard, or to libei on
// KWin, which lacks them. Screens and the overlay use the wlroots client on
// both.
//
// On KWin it triggers the one-time consent prompt now, so the first action
// does not block past the IPC timeout, and elsewhere it is a no-op. It is
// best-effort, with the lazy path as fallback. A session without a keyboard
// still succeeds, so keyboard errors come from the caller.
func WarmWaylandInput() error {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return nil
	}

	hasVirtualPointer, err := wlrootsHasVirtualPointer()
	if err != nil {
		return err
	}

	if hasVirtualPointer {
		return nil
	}

	return libeiEnsure()
}

// HasKeyboard reports whether the Wayland backend can inject keyboard events.
// It is true on wlroots and X11 (XTest), and on KWin reflects whether the
// portal grant included a keyboard, false before any session exists.
func HasKeyboard() bool {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return true
	}

	// The wlroots virtual keyboard needs no portal grant, so check it first.
	hasVKB, err := wlrootsHasVirtualKeyboard()
	if err == nil && hasVKB {
		return true
	}

	has, _ := libeiHasKeyboard()

	return has
}

func waylandMoveCursorToPoint(point image.Point) error {
	hasVirtualPointer, err := wlrootsHasVirtualPointer()
	if err != nil {
		return err
	}

	if hasVirtualPointer {
		return wlrootsMoveCursorToPoint(point)
	}

	err = libeiMoveAbs(point.X, point.Y)
	if err != nil {
		return err
	}

	return wlrootsSetCursor(point)
}

func waylandCursorPosition() (image.Point, error) {
	// The wlroots client holds the cache for both backends.
	return wlrootsCursorPosition()
}

// waylandRefreshCursorPosition re-learns the physical cursor position after
// mouse movement the daemon cannot observe. Compositor IPC (Hyprland) is asked
// first, with the layer-shell discovery pass as the fallback (#1279).
func waylandRefreshCursorPosition(ctx context.Context) error {
	if point, ok := waylandCompositorCursorPosition(ctx); ok {
		return wlrootsSetCursor(point)
	}

	// An IPC attempt that used the deadline must not buy discovery a second
	// budget, since this can run under the mode handler's lock.
	if ctx.Err() != nil {
		return ctx.Err()
	}

	return wlrootsRefreshCursorPosition()
}

func waylandClick(point image.Point, button int) error {
	hasVirtualPointer, err := wlrootsHasVirtualPointer()
	if err != nil {
		return err
	}

	if hasVirtualPointer {
		return wlrootsClick(point, button)
	}

	err = waylandMoveCursorToPoint(point)
	if err != nil {
		return err
	}

	err = libeiButton(button, true)
	if err != nil {
		return err
	}

	// The press landed, so the release must not be lost.
	return libeiButtonRelease(button)
}

func waylandButtonEvent(point image.Point, button int, pressed bool) error {
	hasVirtualPointer, err := wlrootsHasVirtualPointer()
	if err != nil {
		return err
	}

	if hasVirtualPointer {
		return wlrootsButtonEvent(point, button, pressed)
	}

	err = waylandMoveCursorToPoint(point)
	if err != nil {
		return err
	}

	if !pressed {
		// LeftMouseUpAtPoint and drag-release land here, so retry like
		// waylandClick.
		return libeiButtonRelease(button)
	}

	return libeiButton(button, true)
}

func waylandButtonRelease(button int) error {
	hasVirtualPointer, err := wlrootsHasVirtualPointer()
	if err != nil {
		return err
	}

	if hasVirtualPointer {
		return wlrootsButtonRelease(button)
	}

	return libeiButtonRelease(button)
}

func waylandScroll(axis, delta, discrete int) error {
	hasVirtualPointer, err := wlrootsHasVirtualPointer()
	if err != nil {
		return err
	}

	if hasVirtualPointer {
		return wlrootsScroll(axis, delta, discrete)
	}

	return libeiScroll(axis, delta)
}

// waylandScrollContinuous routes one fractional scroll step like waylandScroll.
// Both backends carry fractions natively, so it needs no fallback.
func waylandScrollContinuous(axis int, delta float64) error {
	hasVirtualPointer, err := wlrootsHasVirtualPointer()
	if err != nil {
		return err
	}

	if hasVirtualPointer {
		return wlrootsScrollContinuous(axis, delta)
	}

	return libeiScrollContinuous(axis, delta)
}

// waylandScrollBatch routes a batched scroll like waylandScroll. wlroots
// flushes it in one call. libei has no batch API, so KWin emits one event per
// delta and ignores discretes.
func waylandScrollBatch(axis int, deltas, discretes []int) error {
	hasVirtualPointer, err := wlrootsHasVirtualPointer()
	if err != nil {
		return err
	}

	if hasVirtualPointer {
		return wlrootsScrollBatch(axis, deltas, discretes)
	}

	// Attempt every delta even if one fails, so a device pause mid-batch does
	// not strand a partial scroll. The first error is still reported.
	var firstErr error

	for _, d := range deltas {
		err = libeiScroll(axis, d)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

func waylandModifierEvent(modifier string, isDown bool) error {
	hasVirtualPointer, err := wlrootsHasVirtualPointer()
	if err != nil {
		return err
	}

	if hasVirtualPointer {
		return wlrootsModifierEvent(modifier, isDown)
	}

	keycode, ok := libeiModifierKeycodes[modifier]
	if !ok {
		return derrors.Newf(
			derrors.CodeNotSupported,
			"unsupported modifier %q for libei keyboard injection",
			modifier,
		)
	}

	return libeiKey(keycode, isDown)
}

// waylandKeyEvent presses or releases a key on the virtual keyboard, routing
// through the wlroots virtual keyboard or libei depending on the compositor.
func waylandKeyEvent(keycode uint32, pressed bool) error {
	hasVirtualKeyboard, err := wlrootsHasVirtualKeyboard()
	if err != nil {
		return err
	}

	if hasVirtualKeyboard {
		return wlrootsKey(keycode, pressed)
	}

	return libeiKey(int(keycode), pressed)
}
