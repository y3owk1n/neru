//go:build linux && cgo

package eventtap

import (
	"os"
	"time"

	"github.com/y3owk1n/neru/internal/ui/overlay"
)

const pollingInterval = 10 * time.Millisecond

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

			if key != "" {
				et.dispatchKey(key)
			}
		default:
			time.Sleep(pollingInterval)
		}
	}
}
