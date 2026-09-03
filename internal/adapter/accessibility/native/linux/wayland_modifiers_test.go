//go:build linux

package linux

import (
	"errors"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/action"
)

const (
	ctrlDown = "ctrl down"
	ctrlUp   = "ctrl up"
)

// errVirtualKeyboardRefused is what the recorder answers when a test asks it to
// fail a key event.
var errVirtualKeyboardRefused = errors.New("virtual keyboard refused the event")

// modifierRecorder stands in for the virtual keyboard, remembering every key
// event in order and failing whichever press the test names.
type modifierRecorder struct {
	failPress string
	events    []string
}

func (r *modifierRecorder) event(modifier string, isDown bool) error {
	if isDown && modifier == r.failPress {
		return errVirtualKeyboardRefused
	}

	direction := "up"
	if isDown {
		direction = "down"
	}

	r.events = append(r.events, modifier+" "+direction)

	return nil
}

func withModifierRecorder(t *testing.T, recorder *modifierRecorder) {
	t.Helper()

	original := waylandModifierEvent
	waylandModifierEvent = recorder.event

	t.Cleanup(func() { waylandModifierEvent = original })
}

func assertEvents(t *testing.T, got []string, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("key events = %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("key events = %v, want %v", got, want)
		}
	}
}

// TestPressWaylandModifiers_UnwindsOnlyWhatWentDown is the reason the press
// reports a set instead of trusting the one it was handed.
//
// The dispatcher behind the virtual keyboard refcounts holders and emits a real
// key-up when the count reaches zero, so an unwind that releases the modifier
// whose press just failed, or one it never reached, lets go of a key the user
// may be physically holding rather than canceling out.
func TestPressWaylandModifiers_UnwindsOnlyWhatWentDown(t *testing.T) {
	recorder := &modifierRecorder{failPress: "alt"}
	withModifierRecorder(t, recorder)

	pressed, err := pressWaylandModifiers(
		action.ModShift | action.ModCtrl | action.ModAlt | action.ModCmd,
	)
	if err == nil {
		t.Fatal("pressWaylandModifiers() succeeded, want the recorder's refusal")
	}

	if pressed != 0 {
		t.Fatalf("pressWaylandModifiers() reported %v held after failing, want none", pressed)
	}

	assertEvents(t, recorder.events, []string{
		"shift down", ctrlDown, ctrlUp, "shift up",
	})
}

// TestPressWaylandModifiers_ReportsWhatTheCallerMustRelease pins the set a
// caller defers on: what went down, in press order, and nothing else.
func TestPressWaylandModifiers_ReportsWhatTheCallerMustRelease(t *testing.T) {
	recorder := &modifierRecorder{}
	withModifierRecorder(t, recorder)

	pressed, err := pressWaylandModifiers(action.ModCtrl | action.ModCmd)
	if err != nil {
		t.Fatalf("pressWaylandModifiers() = %v, want no error", err)
	}

	if pressed != action.ModCtrl|action.ModCmd {
		t.Fatalf("pressWaylandModifiers() reported %v, want ctrl and cmd", pressed)
	}

	assertEvents(t, recorder.events, []string{ctrlDown, "cmd down"})
}

// TestReleaseWaylandModifiers_LetsGoOfEveryKeyDespiteAFailure keeps a failed
// release from stranding the modifiers behind it: stopping at the first error
// would leave them held with nobody left to undo them.
func TestReleaseWaylandModifiers_LetsGoOfEveryKeyDespiteAFailure(t *testing.T) {
	recorder := &modifierRecorder{}
	withModifierRecorder(t, recorder)

	waylandModifierEvent = func(modifier string, isDown bool) error {
		if modifier == "cmd" {
			return errVirtualKeyboardRefused
		}

		return recorder.event(modifier, isDown)
	}

	err := releaseWaylandModifiers(action.ModShift | action.ModAlt | action.ModCmd)
	if !errors.Is(err, errVirtualKeyboardRefused) {
		t.Fatalf("releaseWaylandModifiers() = %v, want the recorder's refusal", err)
	}

	assertEvents(t, recorder.events, []string{"alt up", "shift up"})
}
