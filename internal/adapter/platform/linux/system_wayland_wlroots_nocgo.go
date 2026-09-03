//go:build linux && !cgo

package linux

import (
	"image"
	"time"

	"github.com/y3owk1n/neru/internal/derrors"
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

func wlrootsMoveCursorBy(delta image.Point) error {
	_ = delta

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

func wlrootsScroll(axis, delta, discrete int) error {
	_, _, _ = axis, delta, discrete

	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsScrollContinuous(axis int, delta float64) error {
	_, _ = axis, delta

	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsScrollBatch(axis int, deltas, discretes []int) error {
	_, _, _ = axis, deltas, discretes

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

func wlrootsSync(timeout time.Duration) bool {
	_ = timeout

	return false
}

func wlrootsHasVirtualPointer() (bool, error) {
	return false, derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

// wlrootsFocusedAppID is unavailable without CGO — the foreign-toplevel client
// lives in the CGO build. Reports "no focused app_id" so callers fall through
// to their XWayland fallback.
func wlrootsFocusedAppID() (string, bool) {
	return "", false
}

func wlrootsFocusedAppIdentity() (string, string, bool) {
	return "", "", false
}

// wlrootsFocusEventFD is unavailable without CGO — the foreign-toplevel client
// lives in the CGO build. Callers fall back to polling.
func wlrootsFocusEventFD() (int, bool) {
	return -1, false
}

// wlrootsScreenEventFD is unavailable without CGO — the wlroots client lives in
// the CGO build. Callers receive no display-configuration-change events.
func wlrootsScreenEventFD() (int, bool) {
	return -1, false
}

// wlrootsRefreshScreens is a no-op without CGO — there is no wlroots client to
// re-enumerate.
func wlrootsRefreshScreens() {}

func wlrootsSetCursor(point image.Point) error {
	_ = point

	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsRefreshCursorPosition() error {
	return derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsHasVirtualKeyboard() (bool, error) {
	return false, derrors.New(
		derrors.CodeNotSupported,
		"wlroots backend requires CGO-enabled Linux builds",
	)
}

func wlrootsKey(keycode uint32, pressed bool) error {
	_, _ = keycode, pressed

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
