//nolint:testpackage // Tests private mode handler methods.
package modes

import (
	"image"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/state"
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

func TestRunOnExit_DispatchesActionString(t *testing.T) {
	got := make(chan string, 1)
	handler := &Handler{
		logger: zap.NewNop(),
		executeHotkeyAction: func(_, actionStr string) error {
			got <- actionStr

			return nil
		},
	}

	onExit := "exec notify-send done"
	handler.runOnExit(&onExit)

	select {
	case a := <-got:
		if a != onExit {
			t.Fatalf("on-exit dispatched %q, want %q", a, onExit)
		}
	case <-time.After(time.Second):
		t.Fatal("on-exit action was not dispatched")
	}
}

func TestRunOnExit_NilAndEmptyNoop(t *testing.T) {
	called := make(chan struct{}, 1)
	handler := &Handler{
		logger: zap.NewNop(),
		executeHotkeyAction: func(_, _ string) error {
			called <- struct{}{}

			return nil
		},
	}

	handler.runOnExit(nil)

	blank := "   "
	handler.runOnExit(&blank)

	select {
	case <-called:
		t.Fatal("on-exit should not dispatch for nil or blank action")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestCurrentModeOnExit(t *testing.T) {
	hintsExit := "action left_click"
	gridExit := "exec grid-done"
	rgExit := domain.ModeString(domain.ModeRecursiveGrid)

	hintsCtx := &hints.Context{}
	hintsCtx.SetOnExit(&hintsExit)

	gridCtx := &grid.Context{}
	gridCtx.SetOnExit(&gridExit)

	rgCtx := &recursivegrid.Context{}
	rgCtx.SetOnExit(&rgExit)

	appState := state.NewAppState()
	handler := &Handler{
		appState:      appState,
		hints:         &components.HintsComponent{Context: hintsCtx},
		grid:          &components.GridComponent{Context: gridCtx},
		recursiveGrid: &components.RecursiveGridComponent{Context: rgCtx},
	}

	cases := []struct {
		mode domain.Mode
		want string
	}{
		{domain.ModeHints, hintsExit},
		{domain.ModeGrid, gridExit},
		{domain.ModeRecursiveGrid, rgExit},
	}
	for _, testCase := range cases {
		appState.SetMode(testCase.mode)

		got := handler.currentModeOnExit()
		if got == nil || *got != testCase.want {
			t.Fatalf("currentModeOnExit() for %v = %v, want %q",
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
