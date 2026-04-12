//go:build linux && cgo

package eventtap

import (
	"os"

	"github.com/y3owk1n/neru/internal/ui/overlay"
)

func (et *EventTap) runWayland() {
	defer close(et.doneCh)

	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return
	}

	mgr := overlay.Get()
	if mgr == nil {
		return
	}

	keyCh := mgr.WaylandKeyboardChannel()
	if keyCh == nil {
		return
	}

	for {
		select {
		case <-et.stopCh:
			return
		case key, ok := <-keyCh:
			if !ok {
				return
			}

			key = normalizeLinuxKey(key)
			if key == "" {
				continue
			}

			et.dispatchKey(key)
		}
	}
}
