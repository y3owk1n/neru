package mousestate_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/platform/mousestate"
	"github.com/y3owk1n/neru/internal/domain/action"
)

func TestTracker_ZeroValueReportsNothingHeld(t *testing.T) {
	var tracker mousestate.Tracker

	if tracker.AnyDown() {
		t.Error("AnyDown() = true on a zero-value tracker, want false")
	}

	for _, button := range action.MouseButtons() {
		if tracker.IsDown(button) {
			t.Errorf("IsDown(%v) = true on a zero-value tracker, want false", button)
		}
	}

	if held := tracker.HeldButtons(); len(held) != 0 {
		t.Errorf("HeldButtons() = %v on a zero-value tracker, want empty", held)
	}
}

func TestTracker_SetDownAndClear(t *testing.T) {
	var tracker mousestate.Tracker

	point := image.Point{X: 12, Y: 34}
	tracker.SetDown(action.ButtonRight, point, action.ModShift)

	if !tracker.IsDown(action.ButtonRight) {
		t.Fatal("IsDown(right) = false after SetDown, want true")
	}

	if tracker.IsDown(action.ButtonLeft) {
		t.Error("IsDown(left) = true, want false: pressing one button must not hold another")
	}

	gotPoint, heldAtPoint := tracker.DownPosition(action.ButtonRight)
	if !heldAtPoint || gotPoint != point {
		t.Errorf("DownPosition(right) = (%v, %v), want (%v, true)", gotPoint, heldAtPoint, point)
	}

	gotModifiers, heldWithModifiers := tracker.DownModifiers(action.ButtonRight)
	if !heldWithModifiers || gotModifiers != action.ModShift {
		t.Errorf("DownModifiers(right) = (%v, %v), want (%v, true)",
			gotModifiers, heldWithModifiers, action.ModShift)
	}

	tracker.Clear(action.ButtonRight)

	if tracker.IsDown(action.ButtonRight) {
		t.Error("IsDown(right) = true after Clear, want false")
	}

	if _, stillHeld := tracker.DownModifiers(action.ButtonRight); stillHeld {
		t.Error("DownModifiers(right) reported held after Clear, want not held")
	}
}

func TestTracker_HeldButtonsOrdering(t *testing.T) {
	var tracker mousestate.Tracker

	// Pressed out of order to prove the result is ordered by button, not by
	// press order — the drag event type is picked from the first entry.
	tracker.SetDown(action.ButtonMiddle, image.Point{}, 0)
	tracker.SetDown(action.ButtonLeft, image.Point{}, 0)

	held := tracker.HeldButtons()

	want := []action.MouseButton{action.ButtonLeft, action.ButtonMiddle}
	if len(held) != len(want) {
		t.Fatalf("HeldButtons() = %v, want %v", held, want)
	}

	for i, button := range want {
		if held[i] != button {
			t.Errorf("HeldButtons()[%d] = %v, want %v", i, held[i], button)
		}
	}
}

func TestTracker_ClearAll(t *testing.T) {
	var tracker mousestate.Tracker

	for _, button := range action.MouseButtons() {
		tracker.SetDown(button, image.Point{}, 0)
	}

	if !tracker.AnyDown() {
		t.Fatal("AnyDown() = false with every button held, want true")
	}

	tracker.ClearAll()

	if tracker.AnyDown() {
		t.Error("AnyDown() = true after ClearAll, want false")
	}

	if held := tracker.HeldButtons(); len(held) != 0 {
		t.Errorf("HeldButtons() = %v after ClearAll, want empty", held)
	}
}

func TestTracker_SetDownOverwritesPreviousPress(t *testing.T) {
	var tracker mousestate.Tracker

	tracker.SetDown(action.ButtonLeft, image.Point{X: 1, Y: 1}, action.ModCmd)
	tracker.SetDown(action.ButtonLeft, image.Point{X: 2, Y: 2}, action.ModAlt)

	gotPoint, _ := tracker.DownPosition(action.ButtonLeft)
	if want := (image.Point{X: 2, Y: 2}); gotPoint != want {
		t.Errorf("DownPosition(left) = %v, want %v", gotPoint, want)
	}

	gotModifiers, _ := tracker.DownModifiers(action.ButtonLeft)
	if gotModifiers != action.ModAlt {
		t.Errorf("DownModifiers(left) = %v, want %v", gotModifiers, action.ModAlt)
	}
}
