// XKB translation for evdev key codes: mapping captured codes to key and
// modifier names under the active keyboard layout.

//go:build linux && cgo

package linux

/*
#include "../../platform/linux/evdev.h"
#include "../../platform/linux/wayland_keymap.h"
*/
import "C"
import "unsafe"

func (et *EventTap) xkbEvdevKeyName(capture *waylandEvdevCapture, code uint16) string {
	if capture == nil || capture.xkbState == nil {
		return evdevKeyName(code)
	}

	var buf [64]C.char
	if C.neru_xkb_state_key_get_name(
		(*C.neru_xkb_state)(capture.xkbState),
		C.uint16_t(code),
		&buf[0],
		64,
	) == 0 {
		return C.GoString(&buf[0])
	}

	return evdevKeyName(code)
}

// xkbStateModifierName returns the canonical evdev modifier name for the
// given scan code as resolved by the XKB keymap, or empty string when the
// key is not a modifier.  When XKB remaps a physical modifier to a different
// function (e.g. ctrl:swapcaps makes Caps Lock act as Control), this returns
// the remapped modifier name so the handler tracks the correct modifier.
func (et *EventTap) xkbStateModifierName(capture *waylandEvdevCapture, code uint16) string {
	if capture == nil || capture.xkbState == nil {
		return evdevModifierName(code)
	}
	key := et.xkbEvdevKeyName(capture, code)
	if key == "" {
		return evdevModifierName(code)
	}
	switch key {
	case "Shift_L", "Shift_R":
		return evdevModifierShift
	case "Control_L", "Control_R":
		return evdevModifierCtrl
	case "Alt_L", "Alt_R":
		return evdevModifierAlt
	case "Meta_L", "Meta_R", "Super_L", "Super_R", "Hyper_L", "Hyper_R":
		return evdevModifierCmd
	}

	return ""
}

// refreshEvdevXkbState rebuilds xkb_state on activation so lock modifiers and
// layout group reflect the current compositor state, then syncs LED state from
// the devices.
func (et *EventTap) refreshEvdevXkbState(capture *waylandEvdevCapture) {
	if capture.xkbState != nil {
		C.neru_xkb_state_destroy((*C.neru_xkb_state)(capture.xkbState))
	}

	xkb := C.neru_xkb_state_create()
	capture.xkbState = unsafe.Pointer(xkb)

	if xkb == nil {
		if et.logger != nil {
			et.logger.Error(
				"Failed to initialize Wayland xkb_state; XKB options will be ignored, " +
					"falling back to hardcoded evdev key names",
			)
		}

		return
	}

	numLock := C.int(0)
	capsLock := C.int(0)

	capture.deviceMu.Lock()
	for _, file := range capture.files {
		fd := C.int(file.Fd())
		if C.neru_evdev_led_is_on(fd, C.uint(0)) != 0 {
			numLock = 1
		}
		if C.neru_evdev_led_is_on(fd, C.uint(1)) != 0 {
			capsLock = 1
		}
	}
	capture.deviceMu.Unlock()

	C.neru_xkb_state_sync_leds(xkb, numLock, capsLock)
}
