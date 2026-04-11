//go:build linux && !cgo

package accessibility

import (
	"image"

	"github.com/y3owk1n/neru/internal/core/domain/action"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
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

func wlrootsLeftMouseDownAtPoint(point image.Point, modifiers action.Modifiers) error {
	_, _ = point, modifiers
	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsLeftMouseUpAtPoint(point image.Point, modifiers action.Modifiers) error {
	_, _ = point, modifiers
	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsLeftMouseUp() error {
	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsScrollAtCursor(deltaX, deltaY int) error {
	_, _ = deltaX, deltaY
	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}
