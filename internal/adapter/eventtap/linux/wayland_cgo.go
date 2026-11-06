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

	// Falling back from evdev (no readable /dev/input or writable
	// /dev/uinput), so ask the compositor for keyboard focus. Only the Linux
	// backends can, so it is an optional extension.
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
			// consumeSyntheticModifierEvent is deliberately not called here.
			// zwp_virtual_keyboard_v1_modifiers sets modifier state without
			// wl_keyboard.key events, so synthetic modifiers never reach keyCh.

			et.dispatchKey(key)
		}
	}
}
