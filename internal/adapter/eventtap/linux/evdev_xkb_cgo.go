// XKB translation for evdev key codes: mapping captured codes to key and
// modifier names under the active keyboard layout.
//
// These are methods on the capture rather than on a reader, because the answer
// belongs to the devices and their keymap and not to whoever is reading them.
// Both readers of `/dev/input` need it and must agree: the in-mode event tap
// (evdev_session_cgo.go) and the passive global-hotkey listener
// (global_hotkey_cgo.go). One naming for one physical key is what lets a chord a
// user wrote once match in both — the listener naming keys by raw scan code while
// the tap named them by keymap meant a `[hotkeys]` binding answered a different
// physical key inside a mode than out of it on any layout that is not us.

//go:build linux && cgo

package linux

/*
#include "../../platform/linux/evdev.h"
#include "../../platform/linux/wayland_keymap.h"
*/
import "C"
import "unsafe"

// keyName resolves a scan code to the key name it means under the active
// keyboard layout, falling back to the built-in scan-code table when there is no
// keymap to ask.
//
// The fallback is a real answer rather than a failure: it is the name the code
// carries on a us layout, which is what the table holds.
func (capture *waylandEvdevCapture) keyName(code uint16) string {
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

// xkbKeysymName names a state-resolved keysym by the rule keyName applies to a
// scan code, without needing a keymap: the character it types, else the folded
// keysym name. It exists so that rule can be pinned from a test.
func xkbKeysymName(keysym uint32) string {
	var buf [64]C.char
	if C.neru_xkb_keysym_name(C.uint32_t(keysym), &buf[0], C.size_t(len(buf))) != 0 {
		return ""
	}

	return C.GoString(&buf[0])
}

// modifierName returns the canonical evdev modifier name for the given scan code
// as resolved by the XKB keymap, or empty string when the key is not a modifier.
// When XKB remaps a physical modifier to a different function (e.g. ctrl:swapcaps
// makes Caps Lock act as Control), this returns the remapped modifier name so the
// reader tracks the correct modifier.
func (capture *waylandEvdevCapture) modifierName(code uint16) string {
	if capture == nil || capture.xkbState == nil {
		return evdevModifierName(code)
	}

	key := capture.keyName(code)
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

// feedKey tells xkb_state a key went down or up, so its idea of the lock
// modifiers and the layout group keeps up with the keyboard. A reader that
// resolves names without feeding them gets the names the layout had when the
// state was built and never learns of a change.
func (capture *waylandEvdevCapture) feedKey(code uint16, isDown bool) {
	if capture == nil || capture.xkbState == nil {
		return
	}

	down := C.int(0)
	if isDown {
		down = C.int(1)
	}

	C.neru_xkb_state_key((*C.neru_xkb_state)(capture.xkbState), C.uint16_t(code), down)
}

// refreshXkbState rebuilds xkb_state so lock modifiers and layout group reflect
// the current compositor state, then syncs LED state from the devices.
//
// It is called when a reader starts on the devices — mode activation for the
// tap, Start for the listener — because that is when the layout in force may
// have changed since the last time either asked.
func (capture *waylandEvdevCapture) refreshXkbState() {
	if capture == nil {
		return
	}

	if capture.xkbState != nil {
		C.neru_xkb_state_destroy((*C.neru_xkb_state)(capture.xkbState))
	}

	xkb := C.neru_xkb_state_create()
	capture.xkbState = unsafe.Pointer(xkb)

	if xkb == nil {
		if capture.logger != nil {
			capture.logger.Error(
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
