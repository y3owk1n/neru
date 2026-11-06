// XKB translation for evdev key codes: captured codes to key names in the
// reference layout, and to modifier names in the active one.
//
// These are methods on the capture, not a reader, because both readers of
// `/dev/input`, the in-mode tap (evdev_session_cgo.go) and the global-hotkey
// listener (global_hotkey_cgo.go), must name a physical key the same way, or a
// `[hotkeys]` chord answers a different key inside a mode than outside it on
// any non-us layout.

//go:build linux && cgo

package linux

/*
#include <stdlib.h>
#include "../../platform/linux/evdev.h"
#include "../../platform/linux/wayland_keymap.h"
*/
import "C"

import (
	"sync/atomic"
	"unsafe"

	"go.uber.org/zap"
)

// waylandKeyboardLayouts is the last layout list a Wayland capture published,
// for neru doctor, which asks from a goroutine the capture does not run on.
var waylandKeyboardLayouts atomic.Pointer[KeyboardLayouts]

// keyName resolves a scan code to the name Neru's bindings match against,
// applying the live modifiers in the reference layout, the first ASCII-capable
// layout of the keymap. On us,ru with Russian active, the key left of W still
// matches q. Without a keymap it falls back to the built-in scan-code table,
// which holds the us names and is a real answer, not a failure.
func (capture *waylandEvdevCapture) keyName(code uint16) string {
	if capture == nil || capture.xkbState == nil {
		return evdevKeyName(code)
	}

	capture.applyRequestedLayout()

	var buf [64]C.char
	if C.neru_xkb_state_key_get_command_name(
		(*C.neru_xkb_state)(capture.xkbState),
		C.uint16_t(code),
		&buf[0],
		C.size_t(len(buf)),
	) == 0 {
		return C.GoString(&buf[0])
	}

	return evdevKeyName(code)
}

// applyRequestedLayout hands the layout general.kb_layout_to_use forces to the
// xkb state when it changed. It runs on the capture's goroutine before every
// key it names, at the cost of one atomic load.
func (capture *waylandEvdevCapture) applyRequestedLayout() {
	requested := requestedKeyboardLayout.Load()
	if requested == capture.appliedLayout {
		return
	}

	capture.appliedLayout = requested

	var name string
	if requested != nil {
		name = *requested
	}

	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	state := (*C.neru_xkb_state)(capture.xkbState)
	if C.neru_xkb_state_set_reference_layout(state, cName) == 0 && capture.logger != nil {
		capture.logger.Warn(
			"Configured keyboard layout is not in the compositor's keymap; using automatic selection",
			zap.String("layout_id", name),
			zap.Int("layouts", int(C.neru_xkb_state_layout_count(state))),
		)
	}

	waylandKeyboardLayouts.Store(capture.keyboardLayouts())
}

// keyboardLayouts lists the keymap's layouts by XKB name and names the
// reference one.
func (capture *waylandEvdevCapture) keyboardLayouts() *KeyboardLayouts {
	state := (*C.neru_xkb_state)(capture.xkbState)
	layouts := &KeyboardLayouts{Unmatched: C.neru_xkb_state_reference_found(state) == 0}

	for layout := range int(C.neru_xkb_state_layout_count(state)) {
		name := C.neru_xkb_state_layout_name(state, C.int(layout))
		if name == nil {
			continue
		}

		layouts.Names = append(layouts.Names, C.GoString(name))
		if int(C.neru_xkb_state_reference_index(state)) == layout {
			layouts.Reference = C.GoString(name)
		}
	}

	return layouts
}

// liveKeyName resolves a scan code in the layout the keyboard state is in.
// modifierName uses it because an XKB remap such as ctrl:swapcaps belongs to
// the layout that carries it.
func (capture *waylandEvdevCapture) liveKeyName(code uint16) string {
	var buf [64]C.char
	if C.neru_xkb_state_key_get_name(
		(*C.neru_xkb_state)(capture.xkbState),
		C.uint16_t(code),
		&buf[0],
		C.size_t(len(buf)),
	) != 0 {
		return ""
	}

	return C.GoString(&buf[0])
}

// xkbKeysymName names a state-resolved keysym by keyName's rule, without a
// keymap: the character it types, else the folded keysym name. It exists so a
// test can pin that rule.
func xkbKeysymName(keysym uint32) string {
	var buf [64]C.char
	if C.neru_xkb_keysym_name(C.uint32_t(keysym), &buf[0], C.size_t(len(buf))) != 0 {
		return ""
	}

	return C.GoString(&buf[0])
}

// modifierName returns the canonical modifier name for a scan code as the XKB
// keymap resolves it, or "" when the key is not a modifier. An XKB remap such
// as ctrl:swapcaps yields the remapped name, so the reader tracks the right
// modifier.
func (capture *waylandEvdevCapture) modifierName(code uint16) string {
	if capture == nil || capture.xkbState == nil {
		return evdevModifierName(code)
	}

	key := capture.liveKeyName(code)
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

// feedKey tells xkb_state a key went down or up, so its lock modifiers and
// layout group keep up. Without it, names stay as they were when the state was
// built.
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

// pollKeymap reads anything the compositor sent since the last call, without
// blocking, and rebuilds the state when the keymap was replaced or the
// connection is gone, so resolved names follow a layout changed mid-session.
//
// It reports whether the state is fresh. A fresh state has no key history, so
// the caller feeds back what is held.
func (capture *waylandEvdevCapture) pollKeymap() bool {
	// A state that never came up already said so once. Do not repeat it every
	// two seconds.
	if capture == nil || capture.xkbState == nil {
		return false
	}

	// Applied here too, so a forced layout reaches neru doctor without a key
	// press.
	capture.applyRequestedLayout()

	switch C.neru_xkb_state_dispatch((*C.neru_xkb_state)(capture.xkbState)) {
	case 0:
		return false
	case 1:
		// The new keymap may lack the forced layout, or number it differently.
		capture.appliedLayout = nil
		capture.applyRequestedLayout()

		// A fresh state's lock modifiers are stale. Read them off the devices.
		capture.syncLeds()
	default:
		capture.refreshXkbState()
	}

	return true
}

// refreshXkbState rebuilds xkb_state from the compositor's keymap, then syncs
// lock modifiers from the devices' LEDs. It runs when the proxy starts and
// whenever pollKeymap finds the connection gone.
func (capture *waylandEvdevCapture) refreshXkbState() {
	if capture == nil {
		return
	}

	if capture.xkbState != nil {
		C.neru_xkb_state_destroy((*C.neru_xkb_state)(capture.xkbState))
	}

	xkb := C.neru_xkb_state_create()
	capture.xkbState = unsafe.Pointer(xkb)
	capture.appliedLayout = nil

	// A failed rebuild leaves no layouts, so neru doctor must not keep showing
	// the last keymap's.
	waylandKeyboardLayouts.Store(nil)

	if xkb == nil {
		if capture.logger != nil {
			capture.logger.Error(
				"Failed to initialize Wayland xkb_state; XKB options will be ignored, " +
					"falling back to hardcoded evdev key names",
			)
		}

		return
	}

	capture.applyRequestedLayout()
	capture.syncLeds()
}

// syncLeds sets the xkb state's lock modifiers from the devices' LEDs, the
// kernel's word on NumLock and CapsLock when the state has no key history.
func (capture *waylandEvdevCapture) syncLeds() {
	if capture == nil || capture.xkbState == nil {
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

	C.neru_xkb_state_sync_leds((*C.neru_xkb_state)(capture.xkbState), numLock, capsLock)
}
