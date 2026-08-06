package modes

import (
	"context"
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	componentrecursivegrid "github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/domain/state"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

func TestHandleRecursiveGridKey_CompleteSelectionDoesNotMoveWhenCursorFollowSelectionDisabled(
	t *testing.T,
) {
	moveCount := 0

	handler := newHandlerWithState(handlerState{
		config: &config.Config{
			RecursiveGrid: config.RecursiveGridConfig{
				Enabled:       true,
				GridCols:      2,
				GridRows:      2,
				Keys:          "uijk",
				MinSizeWidth:  25,
				MinSizeHeight: 25,
				MaxDepth:      0,
				Hotkeys:       map[string]config.StringOrStringArray{},
				Layers:        nil,
				UI:            config.RecursiveGridUI{},
			},
		},
		logger: zap.NewNop(),
		actionService: services.NewActionService(
			&portmocks.MockAccessibilityPort{},
			&portmocks.MockOverlayPort{},
			&portmocks.MockSystemPort{
				MoveCursorToPointFunc: func(_ context.Context, _ image.Point, _ bool) error {
					moveCount++

					return nil
				},
			},
			zap.NewNop(),
		),
		recursiveGrid: &components.RecursiveGridComponent{
			Context: &componentrecursivegrid.Context{},
		},
		screenBounds: image.Rect(0, 0, 100, 100),
	})

	handler.initializeRecursiveGridManager(image.Rect(0, 0, 100, 100))
	handler.recursiveGrid.Context.SetCursorFollowSelection(false)

	handler.handleRecursiveGridKey("u")

	if moveCount != 0 {
		t.Fatalf("handleRecursiveGridKey() moved cursor %d times, want 0", moveCount)
	}

	selection, ok := handler.recursiveGrid.Context.SelectionPoint()
	if !ok {
		t.Fatal("expected final selection point to be stored")
	}

	if selection != (image.Point{X: 25, Y: 25}) {
		t.Fatalf("stored selection = %v, want (25,25)", selection)
	}
}

func TestResetCurrentMode_RecursiveGridPreservesHoldMode(t *testing.T) {
	moveCount := 0

	appState := state.NewAppState()
	appState.SetMode(domain.ModeRecursiveGrid)

	handler := newHandlerWithState(handlerState{
		appState: appState,
		config: &config.Config{
			RecursiveGrid: config.RecursiveGridConfig{
				Enabled:       true,
				GridCols:      2,
				GridRows:      2,
				Keys:          "uijk",
				MinSizeWidth:  25,
				MinSizeHeight: 25,
				MaxDepth:      10,
				Hotkeys:       map[string]config.StringOrStringArray{},
				UI:            config.RecursiveGridUI{},
			},
		},
		logger: zap.NewNop(),
		actionService: services.NewActionService(
			&portmocks.MockAccessibilityPort{},
			&portmocks.MockOverlayPort{},
			&portmocks.MockSystemPort{
				MoveCursorToPointFunc: func(_ context.Context, _ image.Point, _ bool) error {
					moveCount++

					return nil
				},
			},
			zap.NewNop(),
		),
		overlayPort: &portmocks.MockOverlayPort{},
		recursiveGrid: &components.RecursiveGridComponent{
			Context: &componentrecursivegrid.Context{},
		},
		screenBounds: image.Rect(0, 0, 100, 100),
	})

	handler.initializeRecursiveGridManager(image.Rect(0, 0, 100, 100))
	handler.recursiveGrid.Context.SetCursorFollowSelection(false)

	handler.ResetCurrentMode()

	if moveCount != 0 {
		t.Fatalf("ResetCurrentMode() moved cursor %d times, want 0", moveCount)
	}

	selection, ok := handler.recursiveGrid.Context.SelectionPoint()
	if !ok {
		t.Fatal("expected reset to store the center selection point")
	}

	if selection != (image.Point{X: 50, Y: 50}) {
		t.Fatalf("stored selection after reset = %v, want (50,50)", selection)
	}
}

// TestApplyRecursiveGridFlags_TellsAbsentOnExitFromEmptyOne is the same
// --on-exit contract as grid's, asserted on the mode that stores its own copy
// of it.
func TestApplyRecursiveGridFlags_TellsAbsentOnExitFromEmptyOne(t *testing.T) {
	stored := []string{"exec done"}

	tests := []struct {
		name      string
		onExit    []string
		isRefresh bool
		want      int
	}{
		{name: "absent on a refresh keeps the stored steps", isRefresh: true, want: 1},
		{
			name:      "given but empty on a refresh clears them",
			onExit:    []string{},
			isRefresh: true,
			want:      0,
		},
		{name: "absent on a fresh activation clears them", want: 0},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := &componentrecursivegrid.Context{}
			ctx.SetOnExit(stored)

			applyRecursiveGridFlags(
				ctx,
				modecmd.Activation{OnExit: testCase.onExit},
				testCase.isRefresh,
				true,
			)

			if len(ctx.OnExit()) != testCase.want {
				t.Errorf("OnExit = %v, want %d step(s)", ctx.OnExit(), testCase.want)
			}
		})
	}
}
