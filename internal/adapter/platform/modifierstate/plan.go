package modifierstate

import (
	"slices"

	"github.com/y3owk1n/neru/internal/domain/action"
)

// Key is one modifier key on the live keyboard, as a backend reads it back.
//
// Keycode is whatever identifier that backend injects with; this package only
// ever hands it back. A keymap normally carries two keys per modifier — left
// and right — and both count as holding it.
type Key struct {
	// Keycode identifies the key to the backend that read it.
	Keycode uint32
	// Modifier is the modifier this key carries.
	Modifier action.Modifiers
	// Held is whether the keyboard reports the key down right now.
	Held bool
	// Canonical marks the one key per modifier that Neru presses when it has
	// to present that modifier itself. Exactly one key per modifier should
	// carry it; the rest are read-only for planning purposes.
	Canonical bool
}

// Plan is the pair of edits that make the keyboard present exactly the
// requested modifiers for the length of one injection.
type Plan struct {
	// Suppress are keycodes to release before injecting and press again
	// afterwards: keys something is holding whose modifier was not requested.
	Suppress []uint32
	// Press are keycodes to press before injecting and release afterwards:
	// requested modifiers nothing is already holding.
	Press []uint32
}

// PlanFor works out how to present exactly requested, given the live keyboard.
//
// A modifier something is already holding — the user, or a sticky modifier
// Neru posted earlier — is left exactly as it is when it was asked for, because
// a key is one bit of keyboard state rather than a count: pressing it a second
// time makes our release afterwards announce that the user let go while they
// are still holding it. The same reasoning is why what is held and *not* asked
// for has to be released rather than merely not pressed.
//
// A key the keymap has no keycode for is not injectable and is skipped, and a
// keycode two names share is read once, under the first name that claimed it —
// releasing one key twice would need two presses to undo.
func PlanFor(keys []Key, requested action.Modifiers) Plan {
	var (
		plan      Plan
		presented action.Modifiers
	)

	readable := distinctKeys(keys)

	for _, key := range readable {
		if !key.Held {
			continue
		}

		if requested.Has(key.Modifier) {
			presented |= key.Modifier

			continue
		}

		plan.Suppress = append(plan.Suppress, key.Keycode)
	}

	for _, key := range readable {
		if key.Canonical && requested.Has(key.Modifier) && !presented.Has(key.Modifier) {
			plan.Press = append(plan.Press, key.Keycode)
			presented |= key.Modifier
		}
	}

	return plan
}

// distinctKeys drops the keys no injection can use: one the keymap answered
// with no keycode, and a second name for a keycode an earlier key already
// claimed.
func distinctKeys(keys []Key) []Key {
	distinct := make([]Key, 0, len(keys))
	seen := make(map[uint32]struct{}, len(keys))

	for _, key := range keys {
		if key.Keycode == 0 {
			continue
		}

		if _, repeat := seen[key.Keycode]; repeat {
			continue
		}

		seen[key.Keycode] = struct{}{}

		distinct = append(distinct, key)
	}

	return distinct
}

// Restore reports which suppressed keys to press again once the injection is
// over, given the keycodes the keyboard reports held at that moment.
//
// Everything suppressed is pressed back by default, because the failure that
// direction — a modifier left released while the user is still holding it —
// silently drops that modifier from everything they do next. The exception is a
// key that reads down again: something pressed it after we let go, so a second
// press would leave a hold our release never answers.
func (p Plan) Restore(held []uint32) []uint32 {
	var restore []uint32

	for _, keycode := range p.Suppress {
		if slices.Contains(held, keycode) {
			continue
		}

		restore = append(restore, keycode)
	}

	return restore
}
