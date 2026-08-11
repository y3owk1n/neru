//go:build linux && !cgo

package linux

import "github.com/y3owk1n/neru/internal/derrors"

// libeiEnsure reports that libei input is unavailable.
//
// This is the KDE Plasma input slot built without CGO. libei injection needs
// it, so every entry point here refuses.
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

func libeiScrollContinuous(axis int, delta float64) error {
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

func LibeiReset() {}
