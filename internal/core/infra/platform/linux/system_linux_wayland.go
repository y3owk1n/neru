//go:build linux

package linux

import "image"

// Exported Wayland input entry points. These route to whichever injection
// backend the running compositor supports (zwlr_virtual_pointer on wlroots, or
// libei via the RemoteDesktop portal on KWin/KDE); see
// system_linux_wayland_input.go for the dispatch.

var globalWlrootsModifierDispatcher = newWlrootsModifierDispatcher(waylandModifierEvent)

// WaylandMoveCursorToPoint moves the pointer to an absolute position.
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

// WaylandScroll sends a scroll event. axis: 0=vertical, 1=horizontal.
// delta is in logical pixels (positive = down/right, negative = up/left).
// discrete is the discrete step count (e.g. +/-1 per logical scroll click).
// Each call emits a single scroll event; callers should loop for larger
// scroll distances.
func WaylandScroll(axis, delta, discrete int) error {
	return waylandScroll(axis, delta, discrete)
}

// WaylandModifierEvent presses or releases a modifier key.
func WaylandModifierEvent(modifier string, isDown bool) error {
	return globalWlrootsModifierDispatcher.event(modifier, isDown)
}
