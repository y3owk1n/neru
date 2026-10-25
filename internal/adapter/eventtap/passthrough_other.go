//go:build !linux

package eventtap

// overlayKeyboardPassthroughAllowed reports false everywhere but Linux, because no
// other backend separates the scroll device from the keyboard grab, so there is
// nothing to allow.
func overlayKeyboardPassthroughAllowed() bool { return false }
