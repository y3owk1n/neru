//go:build linux && !cgo

package linux

import (
	"image"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

func wlrootsScreenBounds() (image.Rectangle, error) {
	return image.Rectangle{}, derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsScreenBoundsByName(name string) (image.Rectangle, bool, error) {
	_ = name

	return image.Rectangle{}, false, derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsScreenNames() ([]string, error) {
	return nil, derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsCursorPosition() (image.Point, error) {
	return image.Point{}, derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsMoveCursorToPoint(point image.Point) error {
	_ = point

	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsClick(point image.Point, button int) error {
	_, _ = point, button

	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsButtonEvent(point image.Point, button int, pressed bool) error {
	_, _, _ = point, button, pressed

	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsButtonRelease(button int) error {
	_ = button

	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsScroll(axis, direction int) error {
	_, _ = axis, direction

	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsModifierEvent(modifier string, isDown bool) error {
	_, _ = modifier, isDown

	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

// Button constants matching the CGo version.
const (
	WlrBtnLeft   = 0x110
	WlrBtnRight  = 0x111
	WlrBtnMiddle = 0x112
)
