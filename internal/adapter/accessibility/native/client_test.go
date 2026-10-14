package native

import (
	"image"
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/action"
)

// releaseHeld is what recordMouse logs when the client releases every held
// button.
const releaseHeld = "release held buttons"

// recordMouse points the platform mouse functions at a recorder for one test
// and reports which of them the client called, in order. held lists the
// buttons the platform reports as held down. The functions are package vars,
// so tests that use this must not run in parallel.
func recordMouse(t *testing.T, held []action.MouseButton) *[]string {
	t.Helper()

	priorEnsureMouseUp := ensureMouseUp
	priorIsDown := IsMouseButtonDown
	priorLeftClick := LeftClickAtPoint
	priorMouseUp := MouseUpAtPoint

	t.Cleanup(func() {
		ensureMouseUp = priorEnsureMouseUp
		IsMouseButtonDown = priorIsDown
		LeftClickAtPoint = priorLeftClick
		MouseUpAtPoint = priorMouseUp
	})

	var calls []string

	ensureMouseUp = func() { calls = append(calls, releaseHeld) }
	IsMouseButtonDown = func(button action.MouseButton) bool { return slices.Contains(held, button) }
	LeftClickAtPoint = func(point image.Point, _ bool, _ action.Modifiers) error {
		calls = append(calls, "left click at "+point.String())

		return nil
	}
	MouseUpAtPoint = func(point image.Point, button action.MouseButton, _ action.Modifiers) error {
		calls = append(calls, button.String()+" up at "+point.String())

		return nil
	}

	return &calls
}

func TestClient_PerformAction_LeftClickEndsAHeldLeftDrag(t *testing.T) {
	point := image.Pt(40, 60)

	tests := []struct {
		name string
		held []action.MouseButton
		want []string
	}{
		{
			name: "a held left button is released where the click lands",
			held: []action.MouseButton{action.ButtonLeft},
			want: []string{"left up at (40,60)", releaseHeld},
		},
		{
			name: "a held right button is released along with a held left one",
			held: []action.MouseButton{action.ButtonLeft, action.ButtonRight},
			want: []string{"left up at (40,60)", releaseHeld},
		},
		{
			name: "a held right button is released before a normal click",
			held: []action.MouseButton{action.ButtonRight},
			want: []string{releaseHeld, "left click at (40,60)"},
		},
		{
			name: "with nothing held the click is a normal click",
			want: []string{releaseHeld, "left click at (40,60)"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			calls := recordMouse(t, testCase.held)

			err := New(nil, nil).PerformAction(action.TypeLeftClick, point, false, 0)
			if err != nil {
				t.Fatalf("PerformAction(left_click) failed: %v", err)
			}

			if !slices.Equal(*calls, testCase.want) {
				t.Errorf("PerformAction(left_click) called %q, want %q", *calls, testCase.want)
			}
		})
	}
}
