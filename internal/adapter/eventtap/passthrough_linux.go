//go:build linux

package eventtap

import "github.com/y3owk1n/neru/internal/adapter/eventtap/linux"

// overlayKeyboardPassthroughAllowed reports whether an overlay may let the
// keyboard through. See Adapter.AllowsOverlayKeyboardPassthrough for why both
// conditions are needed.
func overlayKeyboardPassthroughAllowed() bool {
	return linux.IsUinputScrollAvailable() && !linux.IsWaylandEvdevKeyboardActive()
}
