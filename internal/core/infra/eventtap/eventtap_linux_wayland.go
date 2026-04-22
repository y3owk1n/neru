//go:build linux && cgo

package eventtap

import (
	"os"
	"strings"

	"github.com/y3owk1n/neru/internal/ui/overlay"
)

func (et *EventTap) runWayland() {
	defer close(et.doneCh)

	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return
	}

	if et.runWaylandEvdev() {
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

			if strings.HasPrefix(key, "__modifier_") && !et.stickyToggleEnabled() {
				continue
			}
			// Note: consumeSyntheticModifierEvent is intentionally NOT called here.
			// On Wayland, PostModifierEvent drives zwp_virtual_keyboard_v1_modifiers
			// which sets modifier state directly without producing wl_keyboard.key
			// events. Therefore synthetic modifier events never arrive in keyCh.

			et.dispatchKey(key)
		}
	}
}
