//go:build linux && !cgo

package linux

import (
	"image"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
)

func linuxFocusedApplicationIdentity() (string, int) {
	return "", 0
}

func linuxApplicationBundleIdentifier(pid int) string {
	_ = pid
	return ""
}

func x11MoveMouseToPoint(point image.Point) error {
	return derrors.Newf(
		derrors.CodeNotSupported,
		"X11 mouse movement requires CGO-enabled Linux builds (requested point %v)",
		point,
	)
}

func x11CurrentCursorPosition() image.Point {
	return image.Point{}
}

func x11LeftClickAtPoint(point image.Point, restoreCursor bool, modifiers action.Modifiers) error {
	_, _, _ = point, restoreCursor, modifiers
	return derrors.New(
		derrors.CodeNotSupported,
		"X11 left click requires CGO-enabled Linux builds",
	)
}

func x11RightClickAtPoint(point image.Point, restoreCursor bool, modifiers action.Modifiers) error {
	_, _, _ = point, restoreCursor, modifiers
	return derrors.New(
		derrors.CodeNotSupported,
		"X11 right click requires CGO-enabled Linux builds",
	)
}

func x11MiddleClickAtPoint(
	point image.Point,
	restoreCursor bool,
	modifiers action.Modifiers,
) error {
	_, _, _ = point, restoreCursor, modifiers
	return derrors.New(
		derrors.CodeNotSupported,
		"X11 middle click requires CGO-enabled Linux builds",
	)
}

func x11MouseDownAtPoint(
	point image.Point,
	button action.MouseButton,
	modifiers action.Modifiers,
) error {
	_, _, _ = point, button, modifiers
	return derrors.New(
		derrors.CodeNotSupported,
		"X11 mouse down requires CGO-enabled Linux builds",
	)
}

func x11MouseUpAtPoint(
	point image.Point,
	button action.MouseButton,
	modifiers action.Modifiers,
) error {
	_, _, _ = point, button, modifiers
	return derrors.New(
		derrors.CodeNotSupported,
		"X11 mouse up requires CGO-enabled Linux builds",
	)
}

func x11MouseUp(button action.MouseButton, modifiers action.Modifiers) error {
	_, _ = button, modifiers
	return derrors.New(
		derrors.CodeNotSupported,
		"X11 mouse up requires CGO-enabled Linux builds",
	)
}

// x11ScrollBackendAvailable refuses before the animator is ever started. The
// backend is detected from the environment, which says nothing about cgo, so
// without this a CGO_ENABLED=0 build with DISPLAY set would answer "scrolled"
// and move nothing.
func x11ScrollBackendAvailable() error {
	return derrors.New(
		derrors.CodeNotSupported,
		"X11 scroll requires CGO-enabled Linux builds",
	)
}

func newX11ScrollSession(modifiers action.Modifiers) (scrollSession, error) {
	_ = modifiers

	return nil, x11ScrollBackendAvailable()
}

func x11ScrollAtCursor(deltaX, deltaY int, modifiers action.Modifiers) error {
	_, _, _ = deltaX, deltaY, modifiers
	return derrors.New(
		derrors.CodeNotSupported,
		"X11 scroll requires CGO-enabled Linux builds",
	)
}
