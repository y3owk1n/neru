//go:build linux && !cgo

package linux

import (
	"image"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

func x11CursorPosition() (image.Point, error) {
	return image.Point{}, derrors.New(
		derrors.CodeNotSupported,
		"X11 cursor position requires CGO-enabled Linux builds",
	)
}

func x11MoveCursorToPoint(point image.Point) error {
	return derrors.Newf(
		derrors.CodeNotSupported,
		"X11 cursor movement requires CGO-enabled Linux builds (requested point %v)",
		point,
	)
}

func x11FocusedApplicationPID() (int, error) {
	return 0, derrors.New(
		derrors.CodeNotSupported,
		"X11 focused app inspection requires CGO-enabled Linux builds",
	)
}

func x11FocusedAppID() (string, bool) {
	return "", false
}

func x11ActiveScreenBounds() (image.Rectangle, error) {
	return image.Rectangle{}, derrors.New(
		derrors.CodeNotSupported,
		"X11 screen enumeration requires CGO-enabled Linux builds",
	)
}

func x11ScreenBoundsByName(name string) (image.Rectangle, bool, error) {
	return image.Rectangle{}, false, derrors.Newf(
		derrors.CodeNotSupported,
		"X11 screen lookup requires CGO-enabled Linux builds (name=%q)",
		name,
	)
}

func x11ScreenNames() ([]string, error) {
	return nil, derrors.New(
		derrors.CodeNotSupported,
		"X11 screen enumeration requires CGO-enabled Linux builds",
	)
}
