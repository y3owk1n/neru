//go:build linux && !cgo

package eventtap

import derrors "github.com/y3owk1n/neru/internal/core/errors"

// errUinputScrollNoCGO reports that uinput scroll injection is compiled out.
//
// It carries CodeNotSupported rather than a bare error so callers can tell
// "this build cannot do it" from "the device failed", and degrade with
// derrors.IsNotSupported instead of matching on message text.
func errUinputScrollNoCGO(what string) error {
	return derrors.Newf(
		derrors.CodeNotSupported,
		"%s unavailable: this binary was built with CGO_ENABLED=0",
		what,
	)
}

func (et *EventTap) runWayland() {
	close(et.doneCh)
}

func (et *EventTap) runX11() {
	close(et.doneCh)
}

func postLinuxModifierEvent(_ string, _ bool) bool {
	return false
}

func getUinputScrollFd() (int, error) {
	return 0, errUinputScrollNoCGO("uinput scroll device")
}

func ScrollDeviceScroll(_, _ int) error {
	return errUinputScrollNoCGO("uinput scroll")
}

func ScrollDeviceScrollBatch(_ int, _ []int) error {
	return errUinputScrollNoCGO("uinput scroll batch")
}

func IsUinputScrollAvailable() bool {
	return false
}

// IsWaylandEvdevKeyboardActive is always false without CGO (no evdev grab).
func IsWaylandEvdevKeyboardActive() bool {
	return false
}

func (et *EventTap) closeEvdevCapture() {}
