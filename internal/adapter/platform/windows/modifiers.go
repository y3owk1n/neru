//go:build windows

package windows

import (
	"errors"
	"fmt"
	"sync"

	"github.com/y3owk1n/neru/internal/adapter/platform/modifierstate"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// windowsModifierKeys is every virtual key Neru treats as a modifier, canonical
// key first for each one.
//
// Both sides of the keyboard are here because either of them presents the
// modifier, so suppressing one and leaving the other down suppresses nothing.
// The left key is canonical: it is what SendInput presses when Neru has to
// present a modifier itself, and the generic VK_SHIFT / VK_CONTROL / VK_MENU
// codes are deliberately absent, because GetAsyncKeyState answers them as
// "either side" and a plan would then read one hold twice.
var windowsModifierKeys = []struct {
	key       uint16
	modifier  action.Modifiers
	canonical bool
}{
	{vkLShift, action.ModShift, true},
	{vkRShift, action.ModShift, false},
	{vkLControl, action.ModCtrl, true},
	{vkRControl, action.ModCtrl, false},
	{vkLMenu, action.ModAlt, true},
	{vkRMenu, action.ModAlt, false},
	{vkLWin, action.ModCmd, true},
	{vkRWin, action.ModCmd, false},
}

// isModifierVirtualKey reports whether vk is one of windowsModifierKeys.
func isModifierVirtualKey(vk uint32) bool {
	for _, key := range windowsModifierKeys {
		if uint32(key.key) == vk {
			return true
		}
	}

	return false
}

// modifierKeysFrom reads every modifier key through down, which answers
// whether a virtual key is held right now.
func modifierKeysFrom(down func(vk uint32) bool) []modifierstate.Key {
	keys := make([]modifierstate.Key, 0, len(windowsModifierKeys))

	for _, key := range windowsModifierKeys {
		keys = append(keys, modifierstate.Key{
			Keycode:   uint32(key.key),
			Modifier:  key.modifier,
			Held:      down(uint32(key.key)),
			Canonical: key.canonical,
		})
	}

	return keys
}

// heldModifierKeycodes reports which modifier keys the keyboard holds right now.
func heldModifierKeycodes() []uint32 {
	var held []uint32

	for _, key := range modifierKeysFrom(isVirtualKeyDown) {
		if key.Held {
			held = append(held, key.Keycode)
		}
	}

	return held
}

// modifierHoldMu serializes every hold from the read of the keyboard to the
// release that undoes it. A hold reads process-wide keyboard state and then
// changes it, so two overlapping ones — a mode action and an IPC action can run
// on different goroutines — would each plan against the other's edits: the
// first releases a user-held ctrl, the second reads it up and presses it, the
// first then skips restoring it and the second releases it, and the user's
// physically held ctrl reads up until they tap it again.
var modifierHoldMu sync.Mutex

// releasedDuringHold is the modifier keys the user physically let go of while
// a hold was open, or nil when none is. The keyboard hook fills it and release
// reads it, because the keyboard itself cannot answer the question: a key the
// hold suppressed reads up whether the user is still holding it or not, and
// pressing back one they released latches it with nothing to ever let go.
// That window was one synchronous injection wide until the scroll animator
// started holding modifiers for the length of an animation.
var (
	releasedDuringHoldMu sync.Mutex
	releasedDuringHold   map[uint32]struct{}
)

// noteModifierReleased records a physical modifier key-up seen by the
// keyboard hook. Outside a hold it records nothing.
func noteModifierReleased(vk uint32) {
	releasedDuringHoldMu.Lock()
	defer releasedDuringHoldMu.Unlock()

	if releasedDuringHold != nil {
		releasedDuringHold[vk] = struct{}{}
	}
}

func beginReleaseTracking() {
	releasedDuringHoldMu.Lock()
	defer releasedDuringHoldMu.Unlock()

	releasedDuringHold = make(map[uint32]struct{})
}

func endReleaseTracking() map[uint32]struct{} {
	releasedDuringHoldMu.Lock()
	defer releasedDuringHoldMu.Unlock()

	released := releasedDuringHold
	releasedDuringHold = nil

	return released
}

// modifierHold is one injection's worth of falsified modifier state: what it
// released so the injection would not carry it, and what it pressed so the
// injection would.
type modifierHold struct {
	plan modifierstate.Plan
}

// holdModifiers makes the keyboard present exactly modifiers, and returns the
// hold that undoes it.
//
// A SendInput wheel event carries no modifier field — unlike a CGEvent, which
// takes flags — so whatever the keyboard holds rides along on every injected
// event. A plain scroll_down bound to Ctrl+J arrives as ctrl+scroll, which most
// applications read as zoom (#1483). The only way to present a set is to make
// the keyboard actually hold it: release what is held and was not asked for,
// press what was asked for and is not held, and undo both afterwards.
//
// A key already held that was asked for is left exactly as it is, because a key
// is one bit of OS state rather than a count: pressing it a second time makes
// the release afterwards tell every application the user let go while they are
// still holding it.
//
// A key that fails to inject undoes what came before it, so a partial hold is
// never left latched. The caller must release the hold on every path,
// including failure: a modifier left released while the user is still holding
// it drops that modifier from everything they do next.
//
// Every key this presses or releases carries neruInjectedTag, so the low-level
// keyboard hook hands it on without reading it as the user tapping a modifier.
//
// The hold owns modifierHoldMu until release, on the success path and the
// failure path alike.
func holdModifiers(modifiers action.Modifiers) (modifierHold, error) {
	modifierHoldMu.Lock()
	beginReleaseTracking()

	hold := modifierHold{plan: modifierstate.PlanFor(modifierKeysFrom(isVirtualKeyDown), modifiers)}

	for index, edit := range hold.plan.Suppress {
		err := sendKeyboardInput(uint16(edit.Keycode), true)
		if err != nil {
			hold.plan.Suppress = hold.plan.Suppress[:index]
			hold.plan.Press = nil
			hold.release()

			return modifierHold{}, err
		}
	}

	for index, edit := range hold.plan.Press {
		err := sendKeyboardInput(uint16(edit.Keycode), false)
		if err != nil {
			hold.plan.Press = hold.plan.Press[:index]
			hold.release()

			return modifierHold{}, err
		}
	}

	return hold, nil
}

// release lets go of what the hold pressed and presses back what it
// suppressed, skipping any key that reads down again: something pressed it
// after we let go, and a second press would leave a hold our release never
// answers. A key the user let go of while the hold was open reads up exactly
// as one we suppressed does, so the keyboard hook's record of physical
// releases decides: one the user released stays up.
//
// Errors are dropped: a release that fails has nothing better to try, and
// reporting it would mask the outcome of the action it wraps.
func (h modifierHold) release() {
	defer modifierHoldMu.Unlock()

	released := endReleaseTracking()

	for _, edit := range h.plan.Press {
		_ = sendKeyboardInput(uint16(edit.Keycode), true)
	}

	if len(h.plan.Suppress) == 0 {
		return
	}

	for _, edit := range h.plan.Restore(heldModifierKeycodes()) {
		if _, userReleased := released[edit.Keycode]; userReleased {
			continue
		}

		_ = sendKeyboardInput(uint16(edit.Keycode), false)
	}
}

var errUnknownModifier = errors.New("unknown modifier")

// PostModifierKey presses or releases the canonical key for one modifier, named
// in Neru's vocabulary (shift, ctrl, alt, cmd), as if the user had.
//
// It is what the event tap's PostModifierEvent injects with: a sticky modifier
// is a real key held on the user's behalf, and SendInput is the only way to
// hold one. The event is tagged as Neru's own so the keyboard hook does not
// read it back as a toggle.
func PostModifierKey(modifier string, isDown bool) error {
	var key uint16

	switch modifier {
	case modNameShift:
		key = vkLShift
	case modNameCtrl:
		key = vkLControl
	case modNameAlt:
		key = vkLMenu
	case modNameCmd:
		key = vkLWin
	default:
		return fmt.Errorf("%w: %q", errUnknownModifier, modifier)
	}

	return sendKeyboardInput(key, !isDown)
}
