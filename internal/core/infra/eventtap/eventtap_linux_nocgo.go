//go:build linux && !cgo

package eventtap

// closeEvdevCapture is a no-op on non-cgo Linux builds.
// On cgo+Wayland, it closes the persistent evdev capture.
func (et *EventTap) closeEvdevCapture() {}
