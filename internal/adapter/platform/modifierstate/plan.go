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

// Edit is one key a plan touches: which key to inject, and which modifier that
// key presents.
//
// The modifier travels with the keycode because injecting a modifier key is not
// only a request to the display server — on a backend where an injected key
// event re-enters Neru's own event tap, the tap has to be told which modifier
// is about to move, and the keycode alone does not say. Reading it back off the
// keymap a second time at injection would be a second round trip and a second
// chance to disagree with the plan.
type Edit struct {
	// Keycode identifies the key to the backend that read it.
	Keycode uint32
	// Modifier is the modifier this key presents.
	Modifier action.Modifiers
}

// Plan is the pair of edits that make the keyboard present exactly the
// requested modifiers for the length of one injection.
type Plan struct {
	// Suppress are keys to release before injecting and press again
	// afterwards: keys something is holding whose modifier was not requested.
	Suppress []Edit
	// Press are keys to press before injecting and release afterwards:
	// requested modifiers nothing is already holding.
	Press []Edit
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

		plan.Suppress = append(plan.Suppress, key.edit())
	}

	for _, key := range readable {
		if key.Canonical && requested.Has(key.Modifier) && !presented.Has(key.Modifier) {
			plan.Press = append(plan.Press, key.edit())
			presented |= key.Modifier
		}
	}

	return plan
}

// edit is the injectable half of a key: what to inject, and what injecting it
// presents. Held and Canonical are readings of the keyboard as it was when the
// plan was made, and say nothing once the plan is being applied.
func (k Key) edit() Edit {
	return Edit{Keycode: k.Keycode, Modifier: k.Modifier}
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
func (p Plan) Restore(held []uint32) []Edit {
	var restore []Edit

	for _, edit := range p.Suppress {
		if slices.Contains(held, edit.Keycode) {
			continue
		}

		restore = append(restore, edit)
	}

	return restore
}
