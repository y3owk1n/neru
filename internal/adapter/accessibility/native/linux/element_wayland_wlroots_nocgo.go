//go:build linux && !cgo

package linux

import (
	"image"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
)

func wlrootsFocusedApplicationIdentity() (string, int) { return "", 0 }
func wlrootsApplicationBundleIdentifier(pid int) string {
	_ = pid
	return ""
}

func wlrootsMoveMouseToPoint(point image.Point) error {
	_ = point
	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsCurrentCursorPosition() image.Point { return image.Point{} }

func wlrootsLeftClickAtPoint(
	point image.Point,
	restoreCursor bool,
	modifiers action.Modifiers,
) error {
	_, _, _ = point, restoreCursor, modifiers
	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsRightClickAtPoint(
	point image.Point,
	restoreCursor bool,
	modifiers action.Modifiers,
) error {
	_, _, _ = point, restoreCursor, modifiers
	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsMiddleClickAtPoint(
	point image.Point,
	restoreCursor bool,
	modifiers action.Modifiers,
) error {
	_, _, _ = point, restoreCursor, modifiers
	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsMouseDownAtPoint(
	point image.Point,
	button action.MouseButton,
	modifiers action.Modifiers,
) error {
	_, _, _ = point, button, modifiers
	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsMouseUpAtPoint(
	point image.Point,
	button action.MouseButton,
	modifiers action.Modifiers,
) error {
	_, _, _ = point, button, modifiers
	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsMouseUp(button action.MouseButton) error {
	_ = button
	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

// waylandScrollBackendAvailable refuses before the animator is ever started.
// The backend is detected from the environment, which says nothing about cgo,
// so without this a CGO_ENABLED=0 build in a Wayland session would answer
// "scrolled" and move nothing.
func waylandScrollBackendAvailable() error {
	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func newWaylandScrollSession(modifiers action.Modifiers) (scrollSession, error) {
	_ = modifiers

	return nil, waylandScrollBackendAvailable()
}

func wlrootsScrollAtCursor(deltaX, deltaY int, modifiers action.Modifiers) error {
	_, _, _ = deltaX, deltaY, modifiers
	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}
