//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: x11 xtst
#include "../../../platform/linux/x11_accessibility.h"
*/
import "C"

import "github.com/y3owk1n/neru/internal/domain/action"

// x11KeycodeOffset is how far an X keycode sits above the evdev code of the
// same key, on every server that takes its keymap from evdev or libinput.
const x11KeycodeOffset = 8

// x11FeedKey taps the key with evdev code keycode, presenting exactly
// modifiers, and returns once the server has handled it. Its modifier keys
// are not announced to the event tap, which either holds no grab or has
// stepped aside for this key, so it never reads them.
func x11FeedKey(modifiers action.Modifiers, keycode uint32) error {
	display, err := x11ActionDisplay()
	if err != nil {
		return err
	}

	// Closing syncs, so the server has handled every event when this returns.
	defer C.neru_ax_close_display(display)

	hold := x11HoldModifiersFor(display, modifiers, true)
	defer hold.release()

	// Every evdev code ParseFeedKey answers lands inside X's 8 to 255.
	code := C.uint(keycode + x11KeycodeOffset)
	C.neru_ax_key_event(display, code, 1)
	C.neru_ax_key_event(display, code, 0)

	return nil
}
