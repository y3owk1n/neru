//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: x11 xtst
#include <stdlib.h>
#include "../../platform/linux/x11_eventtap.h"
*/
import "C"

import (
	"os"
	"time"
	"unsafe"

	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/domain/keyvocab"
)

const (
	x11PollingInterval = 10 * time.Millisecond
	x11KeyBufferSize   = 64
	x11BitsPerByte     = 8
)

// x11QueryModifierState queries the X11 server for the current keyboard
// state and returns a linuxModifierState counting any held modifier keys.
// This avoids premature detection arming when modifiers were held at grab
// time — their initial KeyRelease events drive counts from positive toward
// zero rather than from zero into negative territory.
func x11QueryModifierState(display *C.Display) linuxModifierState {
	var state linuxModifierState

	var keymap [32]C.char
	C.XQueryKeymap(display, &keymap[0])

	// Map X11 modifier keysyms → our canonical modifier names.
	type modifierKeysym struct {
		keysym   C.KeySym
		modifier string
	}
	modifierKeysyms := []modifierKeysym{
		{C.XK_Shift_L, evdevModifierShift},
		{C.XK_Shift_R, evdevModifierShift},
		{C.XK_Control_L, evdevModifierCtrl},
		{C.XK_Control_R, evdevModifierCtrl},
		{C.XK_Alt_L, evdevModifierAlt},
		{C.XK_Alt_R, evdevModifierAlt},
		{C.XK_Super_L, evdevModifierCmd},
		{C.XK_Super_R, evdevModifierCmd},
		{C.XK_Meta_L, evdevModifierCmd},
		{C.XK_Meta_R, evdevModifierCmd},
	}

	for _, modKey := range modifierKeysyms {
		keycode := C.XKeysymToKeycode(display, modKey.keysym)
		if keycode == 0 {
			continue
		}
		idx := int(keycode) / x11BitsPerByte
		bit := int(keycode) % x11BitsPerByte
		if idx < 32 && (keymap[idx]>>uint(bit))&1 != 0 {
			state.update(modKey.modifier, true)
		}
	}

	return state
}

func (et *EventTap) runX11() {
	defer close(et.doneCh)

	// Do not attempt an X11 keyboard grab if we are running under Wayland.
	// XWayland grabs conflict with Wayland compositor focus policies and frequently
	// result in the compositor sending synthetic "Escape" or "Cancel" keycodes
	// to forcefully break the unauthorized grab, accidentally quitting modes.
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return
	}

	if os.Getenv("DISPLAY") == "" {
		return
	}

	display := C.neru_eventtap_open()
	if display == nil {
		return
	}
	defer C.neru_eventtap_close(display)

	if C.neru_eventtap_grab_keyboard(display) != C.GrabSuccess {
		return
	}
	defer C.neru_eventtap_ungrab_keyboard(display)

	// Query the actual keyboard state after the grab so that modifiers
	// held at grab time are counted in modState. Without this, initial
	// KeyRelease events would drive counts negative, and allZero() would
	// return true prematurely when only some of the held modifiers have
	// been released — arming detection while others are still held.
	modState := x11QueryModifierState(display)

	for {
		select {
		case <-et.stopCh:
			return
		default:
		}

		if C.neru_eventtap_pending(display) == 0 {
			time.Sleep(x11PollingInterval)

			continue
		}

		var event C.XEvent
		eventType := C.neru_eventtap_next(display, &event)
		if eventType != C.KeyPress && eventType != C.KeyRelease {
			continue
		}

		xkey := (*C.XKeyEvent)(unsafe.Pointer(&event))
		buffer := make([]C.char, x11KeyBufferSize)
		var keysym C.KeySym
		length := C.XLookupString(
			xkey,
			&buffer[0],
			C.int(len(buffer)),
			&keysym,
			nil,
		)

		if modifier := x11ModifierName(keysym); modifier != "" {
			isDown := eventType == C.KeyPress
			modState.update(modifier, isDown)

			if et.consumeSyntheticModifierEvent(modifier, isDown) {
				continue
			}

			if et.stickyToggleEnabled() && et.stickyDetectionArmed() {
				et.dispatchKey(keyvocab.ModifierToggleEvent(modifier, isDown))
			}

			// Re-arm when the modifier state reaches a clean slate, so
			// activation-chord releases are not interpreted as sticky toggles.
			if !isDown && !et.stickyDetectionArmed() && modState.allZero() {
				et.stickyArmDetection()
			}

			continue
		}

		key := x11KeyFromLookup(length, buffer, keysym)
		if key == "" {
			continue
		}

		if eventType == C.KeyRelease {
			if keyUp := keyvocab.KeyUpEvent(key); keyUp != "" {
				et.dispatchKey(keyUp)
			}

			continue
		}

		et.dispatchKey(key)
	}
}

func x11KeyFromLookup(length C.int, buffer []C.char, keysym C.KeySym) string {
	var key string
	if length > 0 {
		key = C.GoStringN(&buffer[0], length)
	} else {
		key = x11KeysymName(keysym)
	}

	return keyvocab.NormalizeKey(key)
}

func x11KeysymName(keysym C.KeySym) string {
	switch keysym {
	case C.XK_Return:
		return evdevKeyNameReturn
	case C.XK_space:
		return "Space"
	case C.XK_Tab:
		return "Tab"
	case C.XK_Escape:
		return evdevKeyNameEscape
	case C.XK_BackSpace:
		return "Backspace"
	case C.XK_Left:
		return evdevKeyNameLeft
	case C.XK_Right:
		return "Right"
	case C.XK_Up:
		return "Up"
	case C.XK_Down:
		return "Down"
	// Like the function keys below, the navigation keys produce no character
	// from XLookupString, so this lookup is the only way they reach the
	// handler. The names match the evdev and Wayland backends.
	case C.XK_Home:
		return evdevKeyNameHome
	case C.XK_End:
		return evdevKeyNameEnd
	case C.XK_Page_Up:
		return evdevKeyNamePageUp
	case C.XK_Page_Down:
		return evdevKeyNamePageDown
	default:
		return x11FunctionKeysymName(keysym)
	}
}

// x11FunctionKeysymName maps the F1-F24 keysyms to their Neru display names.
// XLookupString yields no character for function keys, so they only reach the
// handler through this lookup.
func x11FunctionKeysymName(keysym C.KeySym) string {
	switch keysym {
	case C.XK_F1:
		return evdevKeyNameF1
	case C.XK_F2:
		return evdevKeyNameF2
	case C.XK_F3:
		return evdevKeyNameF3
	case C.XK_F4:
		return evdevKeyNameF4
	case C.XK_F5:
		return evdevKeyNameF5
	case C.XK_F6:
		return evdevKeyNameF6
	case C.XK_F7:
		return evdevKeyNameF7
	case C.XK_F8:
		return evdevKeyNameF8
	case C.XK_F9:
		return evdevKeyNameF9
	case C.XK_F10:
		return evdevKeyNameF10
	case C.XK_F11:
		return evdevKeyNameF11
	case C.XK_F12:
		return evdevKeyNameF12
	case C.XK_F13:
		return evdevKeyNameF13
	case C.XK_F14:
		return evdevKeyNameF14
	case C.XK_F15:
		return evdevKeyNameF15
	case C.XK_F16:
		return evdevKeyNameF16
	case C.XK_F17:
		return evdevKeyNameF17
	case C.XK_F18:
		return evdevKeyNameF18
	case C.XK_F19:
		return evdevKeyNameF19
	case C.XK_F20:
		return evdevKeyNameF20
	case C.XK_F21:
		return evdevKeyNameF21
	case C.XK_F22:
		return evdevKeyNameF22
	case C.XK_F23:
		return evdevKeyNameF23
	case C.XK_F24:
		return evdevKeyNameF24
	default:
		return ""
	}
}

func x11ModifierName(keysym C.KeySym) string {
	switch keysym {
	case C.XK_Shift_L, C.XK_Shift_R:
		return evdevModifierShift
	case C.XK_Control_L, C.XK_Control_R:
		return evdevModifierCtrl
	case C.XK_Alt_L, C.XK_Alt_R:
		return evdevModifierAlt
	case C.XK_Super_L, C.XK_Super_R, C.XK_Meta_L, C.XK_Meta_R:
		return evdevModifierCmd
	default:
		return ""
	}
}

func postLinuxModifierEvent(modifier string, isDown bool) bool {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return linux.WaylandModifierEvent(modifier, isDown) == nil
	}

	if os.Getenv("DISPLAY") == "" {
		return false
	}

	cModifier := C.CString(modifier)
	defer C.free(unsafe.Pointer(cModifier))

	cDown := C.int(0)
	if isDown {
		cDown = C.int(1)
	}

	return C.neru_eventtap_post_modifier(cModifier, cDown) != 0
}
