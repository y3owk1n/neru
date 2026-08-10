//go:build linux

package linux

import (
	"context"
	"image"
	"os"
	"time"

	"github.com/y3owk1n/neru/internal/derrors"
)

// evdev KEY_* codes for the libei modifier keyboard path. KWin's RemoteDesktop
// portal commonly grants only a pointer device, so libeiKey may still report
// these as unsupported.
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

// Release-retry backoff for the libei click path. When KWin pauses the
// RemoteDesktop device between a press and its release, the device needs a
// moment to be resumed before a release can be emitted, so retries are spaced
// out instead of fired back-to-back.
const (
	libeiReleaseRetries = 5
	libeiReleaseBackoff = 100 * time.Millisecond
)

// libeiButtonRelease emits a button release, retrying with backoff while the
// device is paused. A press without a matching release leaves the compositor
// with the button logically held (the next pointer move becomes a drag), so
// the release is worth several attempts across a pause/resume cycle.
func libeiButtonRelease(button int) error {
	err := libeiButton(button, false)

	for attempt := 0; err != nil && attempt < libeiReleaseRetries; attempt++ {
		time.Sleep(libeiReleaseBackoff)

		err = libeiButton(button, false)
	}

	return err
}

// WarmWaylandInput pre-establishes the input backend at daemon startup. This
// file routes input to one of two backends: the wlroots virtual-pointer and
// virtual-keyboard protocols, or libei via the RemoteDesktop portal on KWin,
// which does not implement them. Only input differs by compositor — screens
// and the overlay go through the wlroots client on both.
//
// On wlroots or X11 the warm-up is a cheap no-op. On KWin it triggers the
// one-time "Remote Control" consent prompt now, so the first action does not
// block past the IPC timeout. Best-effort: errors are for logging and the lazy
// path stays as fallback. A session without a keyboard still succeeds, so
// keyboard errors come from the caller, not the warm-up.
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
// On wlroots compositors it always returns true (zwp_virtual_keyboard_v1 is
// available). On KDE/KWin it reflects whether the RemoteDesktop portal grant
// included keyboard capability. Safe to call before any warm-up (returns false
// if no session exists yet). On X11 / non-Wayland sessions it returns true
// (keyboard injection goes through XTest, not through this backend).
func HasKeyboard() bool {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return true
	}

	// wlroots compositors provide a virtual keyboard regardless of any portal
	// grant, so we must check that path first before querying the libei state.
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
	// The cursor cache lives in the wlroots client for both backends; libei
	// moves are mirrored into it by waylandMoveCursorToPoint.
	return wlrootsCursorPosition()
}

// waylandRefreshCursorPosition re-learns the physical cursor position after
// user-driven mouse movement the daemon cannot observe. The compositor's own
// IPC is authoritative and cheap where it exists (Hyprland), so it is asked
// first and mirrored into the wlroots cache; the layer-shell discovery pass
// stays as the fallback for compositors without such a query (#1279).
func waylandRefreshCursorPosition(ctx context.Context) error {
	if point, ok := waylandCompositorCursorPosition(ctx); ok {
		return wlrootsSetCursor(point)
	}

	// An IPC attempt that burned the whole deadline must not buy the discovery
	// pass a second budget: this can run under the mode handler's lock, and the
	// discovery wait is bounded internally, not by this context.
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
		// LeftMouseUpAtPoint and drag-release land here. A lost release
		// leaves the compositor with the button held (the next move becomes
		// a drag), so retry across a pause/resume cycle like waylandClick.
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

// waylandScrollBatch routes a batched scroll through the same backend choice as
// waylandScroll. wlroots compositors flush every event in one wlrootsScrollBatch
// call; KWin/KDE has no virtual pointer and libei has no batch API, so it emits
// one libeiScroll event per delta (mirroring the working single-event
// WaylandScroll path). discretes is honored only on the wlroots path; libei
// scrolls by continuous delta, not discrete steps.
func waylandScrollBatch(axis int, deltas, discretes []int) error {
	hasVirtualPointer, err := wlrootsHasVirtualPointer()
	if err != nil {
		return err
	}

	if hasVirtualPointer {
		return wlrootsScrollBatch(axis, deltas, discretes)
	}

	// Attempt every delta even if one fails: a transient libei hiccup (device
	// pause/resume) mid-batch would otherwise drop the remaining deltas and
	// strand the user at a partial scroll position. The first error is still
	// reported to the caller.
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
