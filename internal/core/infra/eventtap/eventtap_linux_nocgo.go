//go:build linux && !cgo

package eventtap

import "errors"

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
	return 0, errors.New("uinput scroll unavailable (no CGO)")
}
