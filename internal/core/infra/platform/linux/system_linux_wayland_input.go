//go:build linux

package linux

import (
	"image"
	"os"
	"time"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// This file is the single routing seam between Neru's Wayland input requests
// and the two non-overlapping injection backends:
//
//   - zwlr_virtual_pointer_v1 / zwp_virtual_keyboard_v1 (wlroots compositors:
//     Sway, Hyprland, niri, River), implemented in the wlroots client.
//   - libei via the org.freedesktop.portal.RemoteDesktop portal (KWin/KDE,
//     which deliberately does not implement the wlroots input protocols),
//     implemented in the libei client.
//
// Screen enumeration and the overlay still go through the wlroots client on
// both families because KWin does implement zwlr_layer_shell_v1 and
// zxdg_output_manager_v1. Only input differs, so the backend choice lives here
// rather than inside either client. The cursor position is cached in the
// wlroots client; after a libei move we mirror the new position back into that
// cache so CursorPosition and screen resolution stay correct.

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

// WarmWaylandInput pre-establishes the Wayland input backend at daemon startup.
// On a wlroots compositor (or X11/non-Wayland session) it is a cheap no-op. On
// KWin/KDE — where input goes through libei via the RemoteDesktop portal — it
// triggers the one-time "Remote Control" consent prompt now, so the first user
// action does not block on the dialog past the IPC timeout. Best-effort: errors
// (no Wayland session, consent declined) are returned for logging and the lazy
// path remains as a fallback.
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
