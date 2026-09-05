//go:build linux && cgo

package linux

import (
	"os"
	"strings"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	overlaymanager "github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/domain/keyvocab"
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

	// Enable keyboard capture on overlay when falling back from evdev.
	// When the proxy cannot serve a session (no readable /dev/input, or no
	// writable /dev/uinput to re-emit through), we need to explicitly request
	// keyboard focus from the compositor. Only the Linux backends can, so it
	// is an optional extension.
	if capture, ok := mgr.(overlaymanager.KeyboardCaptureController); ok {
		capture.SetKeyboardCaptureEnabled(true)
		defer capture.SetKeyboardCaptureEnabled(false)
	}

	for {
		select {
		case <-et.stopCh:
			return
		case key, ok := <-keyCh:
			if !ok {
				return
			}

			key = keyvocab.NormalizeKey(key)
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
