//go:build linux && !cgo

package eventtap

func (et *EventTap) runWayland() {
	close(et.doneCh)
}

func (et *EventTap) runX11() {
	close(et.doneCh)
}
