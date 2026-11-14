//go:build linux

package linux

import (
	"image"
	"time"
)

var globalWlrootsModifierDispatcher = newWlrootsModifierDispatcher(waylandModifierEvent)

// WaylandMoveCursorToPoint moves the pointer to an absolute position. The entry
// points here route to zwlr_virtual_pointer on wlroots or libei on KWin, with
// the dispatch in system_wayland_input.go.
func WaylandMoveCursorToPoint(point image.Point) error {
	return waylandMoveCursorToPoint(point)
}

// WaylandCursorPosition returns the cached cursor position.
func WaylandCursorPosition() (image.Point, error) {
	return waylandCursorPosition()
}

// WaylandClick performs a full click (press + release) at the given position.
func WaylandClick(point image.Point, button int) error {
	return waylandClick(point, button)
}

// WaylandButtonEvent presses or releases a button at the given position.
func WaylandButtonEvent(point image.Point, button int, pressed bool) error {
	return waylandButtonEvent(point, button, pressed)
}

// WaylandButtonRelease releases a button at the current cursor position.
func WaylandButtonRelease(button int) error {
	return waylandButtonRelease(button)
}

// WaylandScroll sends one scroll event. axis is 0 for vertical and 1 for
// horizontal. delta is in logical pixels (positive is down or right), and
// discrete is the step count.
func WaylandScroll(axis, delta, discrete int) error {
	return waylandScroll(axis, delta, discrete)
}

// WaylandScrollContinuous sends one scroll step of an arbitrary distance in
// WaylandScroll's units. Unlike a discrete step, which a compositor may hold
// back until whole notches accumulate, it is delivered as written, which
// smooth scroll needs.
func WaylandScrollContinuous(axis int, delta float64) error {
	return waylandScrollContinuous(axis, delta)
}

// WlrootsScrollBatch sends multiple scroll events in a single flush. deltas and
// discretes must have the same length. Without a virtual pointer (KDE, GNOME,
// COSMIC) it emits one libei wheel click per entry of discretes.
func WlrootsScrollBatch(axis int, deltas, discretes []int) error {
	return waylandScrollBatch(axis, deltas, discretes)
}

// WaylandModifierEvent presses or releases a modifier key.
func WaylandModifierEvent(modifier string, isDown bool) error {
	return globalWlrootsModifierDispatcher.event(modifier, isDown)
}

// WaylandSyncModifiers waits until the compositor has processed the modifier
// events issued so far, and reports whether it answered within timeout. It is
// the barrier between a virtual-keyboard modifier and a uinput scroll, which
// nothing else orders. On false the caller waits a fixed period. KDE is not a
// caller, because libei has no request to sync against.
func WaylandSyncModifiers(timeout time.Duration) bool {
	return wlrootsSync(timeout)
}

// WaylandKeyEvent presses or releases one key by evdev keycode on the virtual
// keyboard. It injects releases for keys held before an evdev grab and released
// during it, keeping the compositor's key state consistent.
func WaylandKeyEvent(keycode uint32, pressed bool) error {
	return waylandKeyEvent(keycode, pressed)
}

// WaylandFocusedAppID returns the app_id of the focused toplevel, which keys
// per-app config on Wayland. It comes from wlr-foreign-toplevel-management on
// wlroots and KWin, and ext-foreign-toplevel-list plus
// zcosmic_toplevel_info_v1 on cosmic-comp. The bool is false on GNOME, with
// nothing focused, or without CGO.
func WaylandFocusedAppID() (string, bool) {
	return wlrootsFocusedAppID()
}

// WaylandFocusedAppIdentity returns the focused toplevel's app_id and title,
// read under one lock so they describe the same window. The title tells apart
// windows sharing an app_id. The bool is false when nothing is focused.
func WaylandFocusedAppIdentity() (string, string, bool) {
	return wlrootsFocusedAppIdentity()
}

// WaylandFocusedWindowGeometry returns the focused toplevel's global bounds in
// logical pixels on cosmic-comp, and CodeNotSupported elsewhere. found=false
// with a nil error means nothing is focused or the window has no position yet.
func WaylandFocusedWindowGeometry() (image.Rectangle, bool, error) {
	return wlrootsFocusedWindowGeometry()
}
