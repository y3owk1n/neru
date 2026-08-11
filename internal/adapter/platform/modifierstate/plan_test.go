package modifierstate_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/platform/modifierstate"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// keycodes are arbitrary; a plan only ever passes them back.
const (
	shiftLeft    = 50
	controlLeft  = 37
	controlRight = 105
	altLeft      = 64
	superLeft    = 133
)

// TestPlanFor_SuppressesAHeldModifierNobodyAskedFor is the bug this package
// exists for: on a backend with no per-event modifier field, an injected event
// carries whatever the display server records as held. A plain scroll fired
// from a Ctrl+J binding therefore arrives as ctrl+scroll, which most
// applications read as zoom.
func TestPlanFor_SuppressesAHeldModifierNobodyAskedFor(t *testing.T) {
	keys := []modifierstate.Key{
		{Keycode: controlLeft, Modifier: action.ModCtrl, Held: true, Canonical: true},
		{Keycode: shiftLeft, Modifier: action.ModShift, Canonical: true},
	}

	plan := modifierstate.PlanFor(keys, 0)

	if len(plan.Suppress) != 1 || plan.Suppress[0] != controlLeft {
		t.Fatalf("PlanFor suppressed %v, want the held ctrl key %d", plan.Suppress, controlLeft)
	}

	if len(plan.Press) != 0 {
		t.Fatalf("PlanFor pressed %v for an unmodified injection, want nothing", plan.Press)
	}
}

// TestPlanFor_PressesOnlyWhatNothingIsAlreadyHolding pins the other half: a key
// is one bit of keyboard state rather than a count, so pressing a modifier the
// user is already holding means our release afterwards tells every application
// they let go while they are still holding it.
func TestPlanFor_PressesOnlyWhatNothingIsAlreadyHolding(t *testing.T) {
	keys := []modifierstate.Key{
		{Keycode: controlLeft, Modifier: action.ModCtrl, Held: true, Canonical: true},
		{Keycode: controlRight, Modifier: action.ModCtrl},
		{Keycode: shiftLeft, Modifier: action.ModShift, Canonical: true},
	}

	plan := modifierstate.PlanFor(keys, action.ModCtrl|action.ModShift)

	if len(plan.Suppress) != 0 {
		t.Fatalf(
			"PlanFor suppressed %v, want nothing: both modifiers were asked for",
			plan.Suppress,
		)
	}

	if len(plan.Press) != 1 || plan.Press[0] != shiftLeft {
		t.Fatalf("PlanFor pressed %v, want only the unheld shift key %d", plan.Press, shiftLeft)
	}
}

// TestPlanFor_SuppressesEveryKeyCarryingAnUnwantedModifier covers the keymap's
// left and right key for the same modifier: leaving either one down leaves the
// modifier presented.
func TestPlanFor_SuppressesEveryKeyCarryingAnUnwantedModifier(t *testing.T) {
	keys := []modifierstate.Key{
		{Keycode: controlLeft, Modifier: action.ModCtrl, Held: true, Canonical: true},
		{Keycode: controlRight, Modifier: action.ModCtrl, Held: true},
		{Keycode: altLeft, Modifier: action.ModAlt, Held: true, Canonical: true},
	}

	plan := modifierstate.PlanFor(keys, action.ModAlt)

	want := []uint32{controlLeft, controlRight}
	if !equalKeycodes(plan.Suppress, want) {
		t.Fatalf("PlanFor suppressed %v, want %v", plan.Suppress, want)
	}
}

// TestPlanFor_IgnoresAKeyTheKeymapHasNoKeycodeFor keeps an unresolvable keysym
// out of the plan: injecting keycode 0 is not a key press, and re-pressing it
// on the way out would be a second one.
func TestPlanFor_IgnoresAKeyTheKeymapHasNoKeycodeFor(t *testing.T) {
	keys := []modifierstate.Key{
		{Keycode: 0, Modifier: action.ModCtrl, Held: true, Canonical: true},
		{Keycode: 0, Modifier: action.ModCmd, Canonical: true},
	}

	plan := modifierstate.PlanFor(keys, action.ModCmd)

	if len(plan.Suppress) != 0 || len(plan.Press) != 0 {
		t.Fatalf("PlanFor planned %v / %v for keycode 0, want nothing", plan.Suppress, plan.Press)
	}
}

// TestPlanFor_ReadsAKeycodeOnceWhenTwoNamesShareIt covers a keymap that answers
// two modifier names with the same key — X11 layouts routinely resolve Meta
// onto the Alt or Super key. Reading it twice would release it twice and press
// it back twice, and the two readings can disagree about whether it was even
// requested.
func TestPlanFor_ReadsAKeycodeOnceWhenTwoNamesShareIt(t *testing.T) {
	keys := []modifierstate.Key{
		{Keycode: altLeft, Modifier: action.ModAlt, Held: true, Canonical: true},
		{Keycode: altLeft, Modifier: action.ModCmd, Held: true},
	}

	plan := modifierstate.PlanFor(keys, 0)

	want := []uint32{altLeft}
	if !equalKeycodes(plan.Suppress, want) {
		t.Fatalf("PlanFor suppressed %v, want %v", plan.Suppress, want)
	}
}

// TestPlanFor_PressesAModifierWhoseOnlyHeldKeyIsBeingSuppressed catches the
// interaction between the two halves: a key read as holding cmd is no use for
// presenting cmd if it is also the alt key we are about to release. Something
// still has to present what was asked for.
func TestPlanFor_PressesAModifierWhoseOnlyHeldKeyIsBeingSuppressed(t *testing.T) {
	keys := []modifierstate.Key{
		{Keycode: altLeft, Modifier: action.ModAlt, Held: true, Canonical: true},
		{Keycode: altLeft, Modifier: action.ModCmd, Held: true},
		{Keycode: superLeft, Modifier: action.ModCmd, Canonical: true},
	}

	plan := modifierstate.PlanFor(keys, action.ModCmd)

	if !equalKeycodes(plan.Suppress, []uint32{altLeft}) {
		t.Fatalf("PlanFor suppressed %v, want the held alt key %d", plan.Suppress, altLeft)
	}

	if !equalKeycodes(plan.Press, []uint32{superLeft}) {
		t.Fatalf("PlanFor pressed %v, want the super key %d to present the requested cmd",
			plan.Press, superLeft)
	}
}

// TestPlan_Restore_PressesBackEveryKeyItSuppressed is the half that must not
// fail: a scroll that leaves ctrl released while the user is still holding it
// is worse than the modifier leak the suppression exists to stop.
func TestPlan_Restore_PressesBackEveryKeyItSuppressed(t *testing.T) {
	plan := modifierstate.Plan{Suppress: []uint32{controlLeft, altLeft}}

	got := plan.Restore(nil)

	want := []uint32{controlLeft, altLeft}
	if !equalKeycodes(got, want) {
		t.Fatalf("Restore() = %v, want %v", got, want)
	}
}

// TestPlan_Restore_SkipsAKeyThatIsDownAgain covers the state changing mid
// injection: a key that reads down was pressed by someone else after we let go
// of it, and pressing it again would leave a press our release never answers.
func TestPlan_Restore_SkipsAKeyThatIsDownAgain(t *testing.T) {
	plan := modifierstate.Plan{Suppress: []uint32{controlLeft, altLeft}}

	got := plan.Restore([]uint32{altLeft, shiftLeft})

	want := []uint32{controlLeft}
	if !equalKeycodes(got, want) {
		t.Fatalf("Restore() = %v, want %v", got, want)
	}
}

func equalKeycodes(got, want []uint32) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}
