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
// first for each one.
//
// Both sides of the keyboard are here because either of them presents the
// modifier, so suppressing one and leaving the other down suppresses nothing.
// Meta comes last: on most layouts XKeysymToKeycode answers it with a keycode
// another entry already claimed, and the plan reads a keycode once, under the
// first modifier that named it.
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
}

// x11HoldModifiers makes the X server present exactly modifiers, and returns
// the hold that undoes it.
//
// An X11 button event carries the server's live modifier state rather than a
// set the caller chooses, so a modifier the user is physically holding rides
// along on every injected event — a plain scroll_down bound to Ctrl+J arrives
// as ctrl+scroll, which most applications read as zoom. macOS has
// CGEventSetFlags for this; here the only way to present a set is to make the
// keyboard actually hold it.
//
// The caller must release the hold on every path, including failure: a
// modifier left released while the user is still holding it drops that modifier
// from everything they do next.
//
// A requested modifier the live keymap has no key for — a layout with no Super
// key, asked for cmd — is presented by nothing and reported by nothing, which
// is what pressing an unresolvable keysym already did before the hold existed.
// The refusal the scroll path does make in that shape is the one
// scroll_modifier_stub_contract_test.go pins: a backend that cannot present a
// modifier at all.
//
// Two costs are accepted deliberately. Reading the keymap is a round trip to
// the server — one per injection, and a second at release only when something
// was actually suppressed — paid on the dispatch goroutine that runs the
// action rather than on the one holding the keyboard grab. And the suppression
// is real X key events: while a mode is active the tap's XGrabKeyboard is what
// they are delivered to, but an action fired with no grab in place presents
// them to the focused window like any other key.
func x11HoldModifiers(display *C.Display, modifiers action.Modifiers) x11ModifierHold {
	hold := x11ModifierHold{
		display: display,
		plan:    modifierstate.PlanFor(x11ModifierKeys(display), modifiers),
	}

	for _, edit := range hold.plan.Suppress {
		hold.inject(edit, false)
	}

	for _, edit := range hold.plan.Press {
		hold.inject(edit, true)
	}

	return hold
}

// inject presses or releases one of the plan's keys, after telling the wired
// sink what is about to go out.
//
// Every key event this file emits goes through here, and it announces before it
// injects rather than after, because the announcement's only reader is a
// keyboard grab on another goroutine that can see the event first otherwise.
// XTest reports nothing back, so a key event the server drops leaves an
// announcement nobody answers — the tap expires those on its own.
func (h x11ModifierHold) inject(edit modifierstate.Edit, pressed bool) {
	recordSyntheticModifier(edit.Modifier, pressed)

	state := C.int(0)
	if pressed {
		state = 1
	}

	C.neru_ax_key_event(h.display, C.uint(edit.Keycode), state)
}

// release lets go of what the hold pressed and presses back what it suppressed,
// against the keyboard as it reads now rather than as it read when the hold was
// taken — the user may have let go of a key or taken hold of another while the
// injection was in flight.
//
// What it pressed goes out unconditionally, where what it suppressed is
// checked. The asymmetry is not a preference: a key this hold pressed reads
// down whether or not the user has since taken hold of it too, so there is
// nothing to check against. The window is one injection, and pressing the same
// modifier inside it is the narrower case of the two.
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

// x11DragModifiers is the hold a mouse-down took, kept until the mouse-up that
// ends the drag comes to undo it.
//
// It is package state because the two calls share nothing else: a press opens a
// display, presses, injects and closes it, and the release opens its own. What
// crosses that boundary is a plan of X keycodes, which name keys on the display
// server rather than on the connection that read them — the same keycode
// releases the same key on the next connection.
var x11DragModifiers modifierstate.Stash

// keepForRelease keeps this hold for the release of button to pick up, instead
// of undoing it when the call that took it returns. button is the X11 button
// number the press injected, which is what the release has to hand back.
//
// A press has to keep presenting its modifiers for as long as the button is
// down, because the drag in between is what carries them: releasing them at the
// end of the press would make a shift+drag an unmodified one from the first
// pixel of movement.
//
// That makes the window a whole drag rather than one injection, and stretches
// the tradeoff the hold already carries: a suppressed key the user lets go of
// mid-drag is still pressed back at the end, and reads as held until they press
// and release it once more. Restoring is the safer bias here for the same
// reason it is inside one injection — the opposite one drops a modifier the
// user is still holding out of everything they do next — but the window it
// applies over is as long as the user's drag.
//
// The keyboard is read once, by the press. What the release replays is the
// press's plan rather than a fresh reading, so a modifier the user takes hold
// of or lets go of mid-drag is presented on the release event as it was at the
// press. Re-reading at the release would decide the drag's modifiers twice,
// which is the one thing a drag cannot have.
func (h x11ModifierHold) keepForRelease(button C.uint) {
	x11DragModifiers.Put(uint32(button), h.plan)
}

// x11ResumeModifierHold picks up the hold the press of button kept, so this
// connection can undo what another one did, and takes a fresh hold of modifiers
// when there was no such press. button is the X11 button number, the same one
// the press was injected with.
//
// The fallback is what a release with nothing behind it needs: a bare mouse-up
// action, or one whose press this process never made. It presents the modifiers
// the caller named for the length of the release event and undoes them
// afterwards, which is the plain non-drag shape.
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
