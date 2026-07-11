//go:build linux && !cgo

package linux

import derrors "github.com/y3owk1n/neru/internal/core/errors"

// KDE Plasma Wayland input slot (non-CGO stub). libei input injection requires
// CGO; these stubs keep the Wayland input dispatcher buildable in the
// (unsupported) non-CGO configuration.

func libeiEnsure() error {
	return derrors.New(
		derrors.CodeNotSupported,
		"libei backend requires CGO-enabled Linux builds",
	)
}

func libeiMoveAbs(x, y int) error {
	_, _ = x, y

	return derrors.New(
		derrors.CodeNotSupported,
		"libei backend requires CGO-enabled Linux builds",
	)
}

func libeiButton(button int, pressed bool) error {
	_, _ = button, pressed

	return derrors.New(
		derrors.CodeNotSupported,
		"libei backend requires CGO-enabled Linux builds",
	)
}

func libeiScroll(axis, delta int) error {
	_, _ = axis, delta

	return derrors.New(
		derrors.CodeNotSupported,
		"libei backend requires CGO-enabled Linux builds",
	)
}

func libeiKey(keycode int, pressed bool) error {
	_, _ = keycode, pressed

	return derrors.New(
		derrors.CodeNotSupported,
		"libei backend requires CGO-enabled Linux builds",
	)
}

func libeiHasKeyboard() (bool, bool) {
	return false, false
}
