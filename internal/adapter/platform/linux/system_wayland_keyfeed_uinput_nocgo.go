//go:build linux && !cgo

package linux

import "github.com/y3owk1n/neru/internal/derrors"

// uinputKeyboardAvailable is always false without CGO. The native bridge drives
// the uinput device, so FeedKey falls through to the compositor backends (which
// are themselves CGO-only and report CodeNotSupported here).
func uinputKeyboardAvailable() bool {
	return false
}

func feedKeyUinput(modifiers []string, keycode uint32) error {
	_, _ = modifiers, keycode

	return derrors.New(
		derrors.CodeNotSupported,
		"uinput keyboard requires CGO-enabled Linux builds",
	)
}
