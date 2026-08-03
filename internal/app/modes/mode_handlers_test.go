//nolint:testpackage // Tests private mode handler methods.
package modes

import (
	"image"
	"slices"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
)

func TestExecuteActionAtPoint_NilActionNoop(t *testing.T) {
	handler := &Handler{
		logger:      zap.NewNop(),
		cursorState: state.NewCursorState(),
	}

	handler.executeActionAtPoint(nil, nil, point(10, 10), false, nil)

	if handler.cursorState.WasActionPerformed() {
		t.Fatal("expected no action state change for nil action")
	}
}

func TestRunOnExit_DispatchesEveryStepInOrder(t *testing.T) {
	got := make(chan []string, 1)
	handler := &Handler{
		logger: zap.NewNop(),
		executeActionSequence: func(_ string, steps []string) {
			got <- steps
		},
	}

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
	handler := &Handler{
		logger: zap.NewNop(),
		executeActionSequence: func(_ string, _ []string) {
			called <- struct{}{}
		},
	}

	handler.runOnExit(nil)
	handler.runOnExit([]string{})

	select {
	case <-called:
		t.Fatal("on-exit should not dispatch for nil or empty steps")
	case <-time.After(100 * time.Millisecond):
	}
}

// optsRecordingMode is a minimal Mode used to capture the options an external
// activation passes through ActivateModeWithOptions.
type optsRecordingMode struct {
	modeType domain.Mode
	lastOpts ModeActivationOptions
}

func (m *optsRecordingMode) Activate(opts ModeActivationOptions) { m.lastOpts = opts }
func (m *optsRecordingMode) HandleKey(string)                    {}
func (m *optsRecordingMode) Exit()                               {}
func (m *optsRecordingMode) ModeType() domain.Mode               { return m.modeType }

func TestActivateModeWithOptions_OmittedOnExitClearsStaleCallback(t *testing.T) {
	fake := &optsRecordingMode{modeType: domain.ModeGrid}
	handler := &Handler{
		logger:   zap.NewNop(),
		appState: state.NewAppState(),
		modes:    map[domain.Mode]Mode{domain.ModeGrid: fake},
	}

	// An external activation that omits --on-exit must reach the mode with a
	// non-nil, empty OnExit so the refresh branch clears any stored steps
	// rather than preserving them.
	handler.ActivateModeWithOptions(domain.ModeGrid, ModeActivationOptions{})

	if fake.lastOpts.OnExit == nil {
		t.Fatal("expected omitted --on-exit to be normalized to a non-nil clear value")
	}

	if len(fake.lastOpts.OnExit) != 0 {
		t.Fatalf("expected normalized on-exit to be empty, got %v", fake.lastOpts.OnExit)
	}

	// Explicit --on-exit steps must pass through untouched, in order.
	want := []string{"exec foo", "action sleep 0.1"}
	handler.ActivateModeWithOptions(domain.ModeGrid, ModeActivationOptions{OnExit: want})

	if !slices.Equal(fake.lastOpts.OnExit, want) {
		t.Fatalf("expected on-exit %v to pass through, got %v", want, fake.lastOpts.OnExit)
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
	handler := &Handler{
		appState:      appState,
		hints:         &components.HintsComponent{Context: hintsCtx},
		grid:          &components.GridComponent{Context: gridCtx},
		recursiveGrid: &components.RecursiveGridComponent{Context: rgCtx},
	}

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
