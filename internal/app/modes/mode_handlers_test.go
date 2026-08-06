package modes

import (
	"context"
	"image"
	"reflect"
	"slices"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/domain/state"
)

func TestExecuteActionAtPoint_NilActionNoop(t *testing.T) {
	handler := newHandlerWithState(handlerState{
		logger:      zap.NewNop(),
		cursorState: state.NewCursorState(),
	})

	handler.executeActionAtPoint(nil, nil, point(10, 10), false, nil)

	if handler.cursorState.WasActionPerformed() {
		t.Fatal("expected no action state change for nil action")
	}
}

func TestRunOnExit_DispatchesEveryStepInOrder(t *testing.T) {
	got := make(chan []string, 1)
	handler := newHandlerWithState(handlerState{
		logger: zap.NewNop(),
		executeActionSequence: func(_ string, steps []string) {
			got <- steps
		},
	})

	onExit := []string{"action sleep 0.2", "exec notify-send done"}
	handler.runOnExit(onExit)

	select {
	case steps := <-got:
		if !slices.Equal(steps, onExit) {
			t.Fatalf("on-exit dispatched %v, want %v", steps, onExit)
		}
	case <-time.After(time.Second):
		t.Fatal("on-exit sequence was not dispatched")
	}
}

func TestRunOnExit_NilAndEmptyNoop(t *testing.T) {
	called := make(chan struct{}, 1)
	handler := newHandlerWithState(handlerState{
		logger: zap.NewNop(),
		executeActionSequence: func(_ string, _ []string) {
			called <- struct{}{}
		},
	})

	handler.runOnExit(nil)
	handler.runOnExit([]string{})

	select {
	case <-called:
		t.Fatal("on-exit should not dispatch for nil or empty steps")
	case <-time.After(100 * time.Millisecond):
	}
}

// stepExecFoo is the on-exit step these cases pass around; what it says does
// not matter, only that it survives the trip.
const stepExecFoo = "exec foo"

// activationRecordingMode is a minimal Mode used to capture the activation an
// external command hands to a mode through ActivateMode.
type activationRecordingMode struct {
	modeType domain.Mode
	last     modecmd.Activation
}

func (m *activationRecordingMode) Activate(
	activation modecmd.Activation,
) {
	m.last = activation
}
func (m *activationRecordingMode) HandleKey(string) {}
func (m *activationRecordingMode) Exit()            {}

func (m *activationRecordingMode) ModeType() domain.Mode                                  { return m.modeType }
func (m *activationRecordingMode) RefreshForMonitorMove(context.Context, image.Rectangle) {}

// newHandlerWithRecordingMode builds a handler whose only mode records what it
// was activated with.
func newHandlerWithRecordingMode(mode domain.Mode) (*Handler, *activationRecordingMode) {
	recorder := &activationRecordingMode{modeType: mode}
	handler := newHandlerWithState(handlerState{
		logger:   zap.NewNop(),
		appState: state.NewAppState(),
		modes:    map[domain.Mode]Mode{mode: recorder},
	})

	return handler, recorder
}

// TestActivateMode_HandsTheWholeActivationToTheMode pins that the handler
// forwards the activation rather than rebuilding it. A field-by-field copy is
// what used to drop flags between being read and being applied, so this
// compares the whole value.
//
// Each case carries every flag its mode accepts and nothing it does not, so
// what is asserted is an activation the grammar can actually produce.
func TestActivateMode_HandsTheWholeActivationToTheMode(t *testing.T) {
	act := actionLeftClick
	modifier := keyPartCmd
	strategy := strategyVision
	labelDirection := dirReverse
	depth := 3
	given := true
	holdCursor := false

	tests := []struct {
		name string
		want modecmd.Activation
	}{
		{
			name: "hints, which accepts the element-detection flags",
			want: modecmd.Activation{
				Mode:                  domain.ModeHints,
				Action:                &act,
				Modifier:              &modifier,
				OnExit:                []string{stepExecFoo},
				Repeat:                &given,
				Toggle:                &given,
				Search:                &given,
				HideOnEmptySearch:     &given,
				SplitWord:             &given,
				CursorFollowSelection: &holdCursor,
				FilterRoles:           []string{"AXButton"},
				FilterTextContains:    []string{"OK"},
				Strategy:              &strategy,
				LabelDirection:        &labelDirection,
			},
		},
		{
			name: "recursive grid, the one mode that zooms",
			want: modecmd.Activation{
				Mode:                  domain.ModeRecursiveGrid,
				Action:                &act,
				Modifier:              &modifier,
				OnExit:                []string{stepExecFoo},
				Repeat:                &given,
				Toggle:                &given,
				CursorFollowSelection: &holdCursor,
				ZoomToDepth:           &depth,
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			handler, recorder := newHandlerWithRecordingMode(testCase.want.Mode)

			handler.ActivateMode(testCase.want)

			if !reflect.DeepEqual(recorder.last, testCase.want) {
				t.Fatalf("mode was activated with %+v, want %+v", recorder.last, testCase.want)
			}
		})
	}
}

// TestActivateMode_OnExitAbsentClearsStaleSteps covers the sharpest edge of the
// --on-exit contract as an external command sees it: a fresh command that says
// nothing about --on-exit must not inherit the steps a previous activation
// stored, or a later completed action runs a command the user never asked for.
func TestActivateMode_OnExitAbsentClearsStaleSteps(t *testing.T) {
	handler, recorder := newHandlerWithRecordingMode(domain.ModeGrid)

	// Absent: normalized to a non-nil, empty value so the mode clears what it
	// stored rather than preserving it.
	handler.ActivateMode(modecmd.Activation{Mode: domain.ModeGrid})

	if recorder.last.OnExit == nil {
		t.Fatal("expected omitted --on-exit to be normalized to a non-nil clear value")
	}

	if len(recorder.last.OnExit) != 0 {
		t.Fatalf("expected normalized on-exit to be empty, got %v", recorder.last.OnExit)
	}

	// Given but empty: already the clear value, and it stays one.
	handler.ActivateMode(modecmd.Activation{Mode: domain.ModeGrid, OnExit: []string{}})

	if recorder.last.OnExit == nil || len(recorder.last.OnExit) != 0 {
		t.Fatalf("expected a given-but-empty --on-exit to stay a clear value, got %v",
			recorder.last.OnExit)
	}

	// Explicit --on-exit steps must pass through untouched, in order.
	want := []string{stepExecFoo, "action sleep 0.1"}
	handler.ActivateMode(modecmd.Activation{Mode: domain.ModeGrid, OnExit: want})

	if !slices.Equal(recorder.last.OnExit, want) {
		t.Fatalf("expected on-exit %v to pass through, got %v", want, recorder.last.OnExit)
	}
}

func TestCurrentModeOnExit(t *testing.T) {
	hintsExit := []string{"action left_click", "action restore_cursor_pos"}
	gridExit := []string{"exec grid-done"}
	rgExit := []string{domain.ModeString(domain.ModeRecursiveGrid)}

	hintsCtx := &hints.Context{}
	hintsCtx.SetOnExit(hintsExit)

	gridCtx := &grid.Context{}
	gridCtx.SetOnExit(gridExit)

	rgCtx := &recursivegrid.Context{}
	rgCtx.SetOnExit(rgExit)

	appState := state.NewAppState()
	handler := newHandlerWithState(handlerState{
		appState:      appState,
		hints:         &components.HintsComponent{Context: hintsCtx},
		grid:          &components.GridComponent{Context: gridCtx},
		recursiveGrid: &components.RecursiveGridComponent{Context: rgCtx},
	})

	cases := []struct {
		mode domain.Mode
		want []string
	}{
		{domain.ModeHints, hintsExit},
		{domain.ModeGrid, gridExit},
		{domain.ModeRecursiveGrid, rgExit},
	}
	for _, testCase := range cases {
		appState.SetMode(testCase.mode)

		got := handler.currentModeOnExit()
		if !slices.Equal(got, testCase.want) {
			t.Fatalf("currentModeOnExit() for %v = %v, want %v",
				testCase.mode, got, testCase.want)
		}
	}

	appState.SetMode(domain.ModeScroll)

	if got := handler.currentModeOnExit(); got != nil {
		t.Fatalf("currentModeOnExit() for non-action mode = %v, want nil", got)
	}
}

func point(x, y int) image.Point {
	return image.Point{X: x, Y: y}
}
