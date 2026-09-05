//go:build windows

package windows

import (
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/platform/modifierstate"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// keyboardHolding answers the key-state probe for a keyboard holding exactly
// the given virtual keys.
func keyboardHolding(keys ...uint16) func(uint32) bool {
	return func(vk uint32) bool {
		return slices.Contains(keys, uint16(vk))
	}
}

func keycodes(edits []modifierstate.Edit) []uint32 {
	codes := make([]uint32, 0, len(edits))
	for _, edit := range edits {
		codes = append(codes, edit.Keycode)
	}

	return codes
}

// A plain scroll bound to a ctrl chord has to release the ctrl the user is
// holding for the length of the injection (#1483), and a modified scroll
// fired with the modifier already down must not press it a second time.
func TestModifierKeysFrom_PlansAgainstTheLiveKeyboard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		held         []uint16
		requested    action.Modifiers
		wantSuppress []uint32
		wantPress    []uint32
	}{
		{
			name:         "user-held ctrl is suppressed for a plain scroll",
			held:         []uint16{vkLControl},
			requested:    0,
			wantSuppress: []uint32{vkLControl},
		},
		{
			name:         "right-hand ctrl is suppressed too",
			held:         []uint16{vkRControl},
			requested:    0,
			wantSuppress: []uint32{vkRControl},
		},
		{
			name:      "held ctrl is left alone when asked for",
			held:      []uint16{vkLControl},
			requested: action.ModCtrl,
		},
		{
			name:      "an unheld modifier is pressed on its canonical key",
			requested: action.ModShift,
			wantPress: []uint32{vkLShift},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			plan := modifierstate.PlanFor(
				modifierKeysFrom(keyboardHolding(testCase.held...)),
				testCase.requested,
			)

			if got := keycodes(plan.Suppress); !slices.Equal(got, testCase.wantSuppress) {
				t.Errorf("Suppress = %#x, want %#x", got, testCase.wantSuppress)
			}

			if got := keycodes(plan.Press); !slices.Equal(got, testCase.wantPress) {
				t.Errorf("Press = %#x, want %#x", got, testCase.wantPress)
			}
		})
	}
}

// TestNoteModifierReleased_RecordsOnlyWhileAHoldIsOpen pins the window a
// physical release is remembered in: a key-up the hook sees between a hold's
// open and its release is what keeps that key from being pressed back, and
// one seen outside a hold means nothing.
func TestNoteModifierReleased_RecordsOnlyWhileAHoldIsOpen(t *testing.T) {
	noteModifierReleased(vkLShift)

	if released := endReleaseTracking(); len(released) != 0 {
		t.Fatalf("a release outside a hold was recorded: %v", released)
	}

	beginReleaseTracking()
	noteModifierReleased(vkLShift)

	released := endReleaseTracking()
	if _, ok := released[vkLShift]; !ok || len(released) != 1 {
		t.Fatalf("released = %v, want exactly left shift", released)
	}

	if again := endReleaseTracking(); again != nil {
		t.Fatalf("the record survived its hold: %v", again)
	}
}

// A drag is two calls with the user's movement in between, so the hold the
// press took has to reach the release intact: the same plan, with the lock and
// the release record reopened for it, and let go of in between so a scroll
// fired mid-drag can take them.
func TestModifierHold_KeepForRelease_HandsThePlanToTheMatchingRelease(t *testing.T) {
	plan := modifierstate.Plan{
		Suppress: []modifierstate.Edit{{Keycode: vkLControl, Modifier: action.ModCtrl}},
		Press:    []modifierstate.Edit{{Keycode: vkLShift, Modifier: action.ModShift}},
	}

	modifierHoldMu.Lock()
	beginReleaseTracking()

	modifierHold{plan: plan}.keepForRelease(action.ButtonLeft)

	if !modifierHoldMu.TryLock() {
		t.Fatal("keepForRelease kept the hold lock across the drag")
	}

	modifierHoldMu.Unlock()

	noteModifierReleased(vkLControl)

	if released := endReleaseTracking(); released != nil {
		t.Fatalf("release tracking stayed open across the drag: %v", released)
	}

	resumed, err := resumeModifierHold(action.ButtonLeft, 0)
	if err != nil {
		t.Fatalf("resumeModifierHold: %v", err)
	}

	if !slices.Equal(keycodes(resumed.plan.Suppress), keycodes(plan.Suppress)) ||
		!slices.Equal(keycodes(resumed.plan.Press), keycodes(plan.Press)) {
		t.Fatalf("resumed plan = %+v, want the press's plan %+v", resumed.plan, plan)
	}

	if modifierHoldMu.TryLock() {
		modifierHoldMu.Unlock()
		t.Fatal("resumeModifierHold did not take the hold lock for the release")
	}

	noteModifierReleased(vkLControl)

	released := endReleaseTracking()
	modifierHoldMu.Unlock()

	if _, ok := released[vkLControl]; !ok {
		t.Fatalf("release tracking was not reopened for the release: %v", released)
	}

	if _, held := windowsDragModifiers.Take(uint32(action.ButtonLeft)); held {
		t.Fatal("the press's plan survived its release")
	}
}
