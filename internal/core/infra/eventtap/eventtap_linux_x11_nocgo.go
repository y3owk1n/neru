//go:build linux && !cgo

package eventtap

func (et *EventTap) run() {
	close(et.doneCh)
}
