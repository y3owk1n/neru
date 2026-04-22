//go:build linux

package linux

import "image"

// Exported wrappers for wlroots input injection.
// These delegate to the build-tagged (cgo/nocgo) implementations.

// WlrootsMoveCursorToPoint moves the virtual pointer to an absolute position.
func WlrootsMoveCursorToPoint(point image.Point) error {
	return wlrootsMoveCursorToPoint(point)
}

// WlrootsCursorPosition returns the cached cursor position.
func WlrootsCursorPosition() (image.Point, error) {
	return wlrootsCursorPosition()
}

// WlrootsClick performs a full click (press + release) at the given position.
func WlrootsClick(point image.Point, button int) error {
	return wlrootsClick(point, button)
}

// WlrootsButtonEvent presses or releases a button at the given position.
func WlrootsButtonEvent(point image.Point, button int, pressed bool) error {
	return wlrootsButtonEvent(point, button, pressed)
}

// WlrootsButtonRelease releases a button at the current cursor position.
func WlrootsButtonRelease(button int) error {
	return wlrootsButtonRelease(button)
}

// WlrootsScroll sends a scroll event. axis: 0=vertical, 1=horizontal.
// delta is in logical pixels (positive = down/right, negative = up/left).
// Each call emits a single Wayland axis event; callers should loop for
// larger scroll distances.
func WlrootsScroll(axis, delta int) error {
	return wlrootsScroll(axis, delta)
}

// WlrootsModifierEvent presses or releases a virtual keyboard modifier.
func WlrootsModifierEvent(modifier string, isDown bool) error {
	return wlrootsModifierEvent(modifier, isDown)
}
