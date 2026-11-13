//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: x11 xtst
#include "../../../platform/linux/x11_accessibility.h"
*/
import "C"

import (
	"github.com/y3owk1n/neru/internal/adapter/platform/modifierstate"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// x11ModifierKeysyms is every keysym Neru treats as a modifier, canonical key
// first for each. Both sides are listed because either presents the modifier,
// so suppressing one alone suppresses nothing. Meta comes last because
// XKeysymToKeycode usually answers it with a keycode another entry claimed,
// and the plan reads a keycode once, under the first modifier naming it.
var x11ModifierKeysyms = []struct {
	keysym    C.KeySym
	modifier  action.Modifiers
	canonical bool
}{
	{C.XK_Shift_L, action.ModShift, true},
	{C.XK_Shift_R, action.ModShift, false},
	{C.XK_Control_L, action.ModCtrl, true},
	{C.XK_Control_R, action.ModCtrl, false},
	{C.XK_Alt_L, action.ModAlt, true},
	{C.XK_Alt_R, action.ModAlt, false},
	{C.XK_Super_L, action.ModCmd, true},
	{C.XK_Super_R, action.ModCmd, false},
	{C.XK_Meta_L, action.ModCmd, false},
	{C.XK_Meta_R, action.ModCmd, false},
}

// x11ModifierHold is one injection's worth of falsified modifier state: what it
// released so the injection would not carry it, and what it pressed so the
// injection would.
type x11ModifierHold struct {
	display *C.Display
	plan    modifierstate.Plan

	// unannounced skips telling the event tap about each key, for a caller
	// whose keys the tap never reads (x11FeedKey).
	unannounced bool
}

// x11HoldModifiers makes the X server present exactly modifiers, and returns
// the hold that undoes it.
//
// An X11 button event carries the server's live modifier state, so a modifier
// the user physically holds goes out with the event. A plain scroll_down bound to Ctrl+J
// arrives as ctrl+scroll, which most apps read as zoom. With no equivalent of
// macOS's CGEventSetFlags, the keyboard has to actually hold the set.
//
// The caller must release the hold on every path, including failure. A
// modifier left released while the user holds it drops out of everything they
// do next.
//
// A requested modifier the keymap has no key for (cmd with no Super key) is
// silently not presented. The refusal scroll_modifier_stub_contract_test.go
// pins is for a backend that cannot present a modifier at all.
//
// Two costs are accepted. Reading the keymap is a server round trip on the
// dispatch goroutine, not the grab goroutine, once per injection plus once at
// release if something was suppressed. And the suppression is real X key
// events. With a mode active they reach the tap's XGrabKeyboard, but an action
// fired with no grab presents them to the focused window.
func x11HoldModifiers(display *C.Display, modifiers action.Modifiers) x11ModifierHold {
	return x11HoldModifiersFor(display, modifiers, false)
}

// x11HoldModifiersFor is x11HoldModifiers, with unannounced for a caller
// whose keys never reach the event tap.
func x11HoldModifiersFor(
	display *C.Display,
	modifiers action.Modifiers,
	unannounced bool,
) x11ModifierHold {
	hold := x11ModifierHold{
		display:     display,
		plan:        modifierstate.PlanFor(x11ModifierKeys(display), modifiers),
		unannounced: unannounced,
	}

	for _, edit := range hold.plan.Suppress {
		hold.inject(edit, false)
	}

	for _, edit := range hold.plan.Press {
		hold.inject(edit, true)
	}

	return hold
}

// inject presses or releases one of the plan's keys after announcing it to the
// wired sink. It is this file's only key-event call site. It announces first
// because the reader is a keyboard grab on another goroutine that could see the
// event before a late announcement. XTest reports nothing back, so the tap
// expires announcements for dropped events itself.
//
// A registration names a modifier and direction, not an event, so a user press
// of the same modifier in the one-cgo-call gap here is matched instead of ours.
// XTest has no way to tag the event, which is why this channel exists.
func (h x11ModifierHold) inject(edit modifierstate.Edit, pressed bool) {
	if !h.unannounced {
		recordSyntheticModifier(edit.Modifier, pressed)
	}

	state := C.int(0)
	if pressed {
		state = 1
	}

	C.neru_ax_key_event(h.display, C.uint(edit.Keycode), state)
}

// release lets go of what the hold pressed and presses back what it
// suppressed, against the keyboard as it reads now, since the user may have
// changed keys mid-injection. Releasing what it pressed is unconditional,
// because a key this hold pressed reads down either way and there is nothing
// to check against. Restoring what it suppressed is checked.
func (h x11ModifierHold) release() {
	for _, edit := range h.plan.Press {
		h.inject(edit, false)
	}

	if len(h.plan.Suppress) == 0 {
		return
	}

	for _, edit := range h.plan.Restore(x11HeldModifierKeycodes(h.display)) {
		h.inject(edit, true)
	}
}

// x11DragModifiers is the hold a mouse-down took, kept for the mouse-up that
// ends the drag. Press and release each open their own display, and an X
// keycode names a key on the server rather than the connection that read it,
// so the plan replays on the next connection.
var x11DragModifiers modifierstate.Stash

// keepForRelease keeps this hold for the release of button (the X11 button
// number the press injected) instead of undoing it when the press returns.
// The drag carries the modifiers, so releasing them at the end of the press
// would make a shift+drag unmodified from the first pixel.
//
// That stretches the window to a whole drag. A suppressed key the user lets go
// of mid-drag is still pressed back at the end and reads as held until pressed
// once more. Restoring is still the safer bias, since the opposite drops a
// modifier the user holds out of everything they do next.
//
// The release replays the press's plan rather than re-reading the keyboard,
// because deciding a drag's modifiers twice is the one thing a drag cannot
// have.
func (h x11ModifierHold) keepForRelease(button C.uint) {
	x11DragModifiers.Put(uint32(button), h.plan)
}

// x11ResumeModifierHold picks up the hold the press of button kept, so this
// connection can undo what another did. With no such press (a bare mouse-up,
// or one whose press this process never made) it takes a fresh hold of
// modifiers for the release event alone.
func x11ResumeModifierHold(
	display *C.Display,
	button C.uint,
	modifiers action.Modifiers,
) x11ModifierHold {
	if plan, held := x11DragModifiers.Take(uint32(button)); held {
		return x11ModifierHold{display: display, plan: plan}
	}

	return x11HoldModifiers(display, modifiers)
}

// x11ModifierKeys reads the live keymap and reports every modifier key on it.
func x11ModifierKeys(display *C.Display) []modifierstate.Key {
	var keymap [C.NERU_AX_KEYMAP_BYTES]C.char

	C.neru_ax_query_keymap(display, &keymap[0])

	keys := make([]modifierstate.Key, 0, len(x11ModifierKeysyms))

	for _, entry := range x11ModifierKeysyms {
		keycode := uint32(C.neru_ax_keysym_to_keycode(display, entry.keysym))

		keys = append(keys, modifierstate.Key{
			Keycode:  keycode,
			Modifier: entry.modifier,
			Held: keycode != 0 &&
				C.neru_ax_keycode_is_held(&keymap[0], C.uint(keycode)) != 0,
			Canonical: entry.canonical,
		})
	}

	return keys
}

// x11HeldModifierKeycodes reports which modifier keycodes the keyboard has down
// right now.
func x11HeldModifierKeycodes(display *C.Display) []uint32 {
	var held []uint32

	for _, key := range x11ModifierKeys(display) {
		if key.Held {
			held = append(held, key.Keycode)
		}
	}

	return held
}
