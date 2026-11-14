package mouse

import (
	"context"
	"errors"
	"image"
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// errInjection stands in for a backend's injection failure.
var errInjection = errors.New("injection failed")

// releaseHeld is what the recorder logs when the adapter releases every held
// button.
const releaseHeld = "release held buttons"

// recorder is a backend that logs each call in order. held lists the buttons
// it reports as held down, and scrollErr is what a scroll returns.
type recorder struct {
	held      []action.MouseButton
	scrollErr error
	calls     []string
}

func (r *recorder) backend() backend {
	return backend{
		isDown:     func(button action.MouseButton) bool { return slices.Contains(r.held, button) },
		releaseAll: func() { r.calls = append(r.calls, releaseHeld) },
		click: func(point image.Point, button action.MouseButton, _ action.Modifiers) error {
			r.calls = append(r.calls, button.String()+" click at "+point.String())

			return nil
		},
		down: func(point image.Point, button action.MouseButton, _ action.Modifiers) error {
			r.calls = append(r.calls, button.String()+" down at "+point.String())

			return nil
		},
		up: func(point image.Point, button action.MouseButton, _ action.Modifiers) error {
			r.calls = append(r.calls, button.String()+" up at "+point.String())

			return nil
		},
		move: func(point image.Point) { r.calls = append(r.calls, "move to "+point.String()) },
		scroll: func(deltaX, deltaY int, _ action.Modifiers) error {
			r.calls = append(r.calls, "scroll")

			return r.scrollErr
		},
	}
}

func TestAdapter_PerformActionAtPoint_LeftClickEndsAHeldLeftDrag(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			rec := &recorder{held: testCase.held}

			err := newAdapter(nil, rec.backend()).
				PerformActionAtPoint(t.Context(), action.TypeLeftClick, point, 0)
			if err != nil {
				t.Fatalf("PerformActionAtPoint(left_click) failed: %v", err)
			}

			if !slices.Equal(rec.calls, testCase.want) {
				t.Errorf("calls = %q, want %q", rec.calls, testCase.want)
			}
		})
	}
}

func TestAdapter_PerformActionAtPoint_ToggleFollowsTheHeldButtons(t *testing.T) {
	t.Parallel()

	point := image.Pt(5, 7)

	tests := []struct {
		name string
		held []action.MouseButton
		want []string
	}{
		{name: "a free button is pressed", want: []string{"right down at (5,7)"}},
		{
			name: "a held button is released",
			held: []action.MouseButton{action.ButtonRight},
			want: []string{"right up at (5,7)"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			rec := &recorder{held: testCase.held}

			err := newAdapter(nil, rec.backend()).
				PerformActionAtPoint(t.Context(), action.TypeRightMouseToggle, point, 0)
			if err != nil {
				t.Fatalf("PerformActionAtPoint(right_mouse_toggle) failed: %v", err)
			}

			if !slices.Equal(rec.calls, testCase.want) {
				t.Errorf("calls = %q, want %q", rec.calls, testCase.want)
			}
		})
	}
}

func TestAdapter_PerformActionAtPoint_MoveMouseMovesAndReportsNothing(t *testing.T) {
	t.Parallel()

	rec := &recorder{}

	err := newAdapter(nil, rec.backend()).
		PerformActionAtPoint(t.Context(), action.TypeMoveMouse, image.Pt(3, 4), 0)
	if err != nil {
		t.Fatalf("PerformActionAtPoint(move_mouse) error = %v, want nil", err)
	}

	if want := []string{"move to (3,4)"}; !slices.Equal(rec.calls, want) {
		t.Errorf("calls = %q, want %q", rec.calls, want)
	}
}

func TestAdapter_PerformActionAtPoint_RefusesAScroll(t *testing.T) {
	t.Parallel()

	rec := &recorder{}

	err := newAdapter(nil, rec.backend()).
		PerformActionAtPoint(t.Context(), action.TypeScroll, image.Pt(0, 0), 0)
	if err == nil {
		t.Fatal("PerformActionAtPoint(scroll) error = nil, want a refusal")
	}

	if len(rec.calls) != 0 {
		t.Errorf("calls = %q, want none", rec.calls)
	}
}

func TestAdapter_Scroll_ReleasesHeldButtonsFirst(t *testing.T) {
	t.Parallel()

	rec := &recorder{held: []action.MouseButton{action.ButtonLeft}}

	err := newAdapter(nil, rec.backend()).Scroll(t.Context(), 0, -10, 0)
	if err != nil {
		t.Fatalf("Scroll() error = %v", err)
	}

	if want := []string{releaseHeld, "scroll"}; !slices.Equal(rec.calls, want) {
		t.Errorf("calls = %q, want %q", rec.calls, want)
	}
}

func TestAdapter_Scroll_ReportsFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		scrollErr error
		wantCode  derrors.Code
	}{
		{
			name:      "a refused modifier stays CodeNotSupported",
			scrollErr: derrors.New(derrors.CodeNotSupported, "cannot present ctrl"),
			wantCode:  derrors.CodeNotSupported,
		},
		{
			name:      "any other failure is CodeActionFailed",
			scrollErr: errInjection,
			wantCode:  derrors.CodeActionFailed,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			rec := &recorder{scrollErr: testCase.scrollErr}

			err := newAdapter(nil, rec.backend()).Scroll(t.Context(), 0, -10, action.ModCtrl)
			if got := derrors.GetCode(err); got != testCase.wantCode {
				t.Errorf(
					"Scroll() error code = %v, want %v (error %v)",
					got,
					testCase.wantCode,
					err,
				)
			}
		})
	}
}

// TestAdapter_ReleaseHeldButtons_IsIdempotent pins that the release runs on
// every mode-exit path, so it must be safe when nothing is held.
func TestAdapter_ReleaseHeldButtons_IsIdempotent(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	adapter := newAdapter(nil, rec.backend())

	for attempt := range 3 {
		err := adapter.ReleaseHeldButtons(t.Context())
		if err != nil {
			t.Fatalf("ReleaseHeldButtons() attempt %d error = %v, want nil", attempt+1, err)
		}
	}
}

// TestAdapter_ReleaseHeldButtons_HonorsACanceledContext pins the context
// check, so a canceled cleanup does not reach into the platform layer.
func TestAdapter_ReleaseHeldButtons_HonorsACanceledContext(t *testing.T) {
	t.Parallel()

	rec := &recorder{}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := newAdapter(nil, rec.backend()).ReleaseHeldButtons(ctx)
	if err == nil {
		t.Fatal("ReleaseHeldButtons() on a canceled context error = nil, want an error")
	}

	if len(rec.calls) != 0 {
		t.Errorf("calls = %q, want none", rec.calls)
	}
}
