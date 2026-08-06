package modes

import (
	"context"
	"image"
	"testing"

	"go.uber.org/zap"

	overlaypkg "github.com/y3owk1n/neru/internal/adapter/overlay"
	gridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/render"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

func TestHandleGridModeKey_CompleteSelectionDoesNotMoveWhenCursorFollowSelectionDisabled(
	t *testing.T,
) {
	moveCount := 0

	gridInstance := domainGrid.NewGridWithLabels(
		"ABCD",
		"",
		"",
		image.Rect(0, 0, 100, 100),
		zap.NewNop(),
	)

	manager := domainGrid.NewManager(
		gridInstance,
		3,
		3,
		"asdfghjkl",
		nil,
		nil,
		zap.NewNop(),
	)

	handler := newHandlerWithState(handlerState{
		config: &config.Config{
			Grid: config.GridConfig{
				Enabled:    true,
				Characters: "ABCD",
				Hotkeys:    map[string]config.StringOrStringArray{},
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
		grid: &components.GridComponent{
			Manager: manager,
			Router:  domainGrid.NewRouter(manager, zap.NewNop()),
			Context: &gridcomponent.Context{},
		},
		screenBounds: image.Rect(0, 0, 100, 100),
	})

	handler.grid.Context.SetCursorFollowSelection(false)

	handler.handleGridModeKey("A")
	handler.handleGridModeKey("A")
	handler.handleGridModeKey("A")

	if moveCount != 0 {
		t.Fatalf("handleGridModeKey() moved cursor %d times, want 0", moveCount)
	}

	if _, ok := handler.grid.Context.SelectionPoint(); !ok {
		t.Fatal("expected final selection point to be stored")
	}
}

func TestHandleGridModeKey_EnteringSubgridDoesNotMoveWhenCursorFollowSelectionDisabled(
	t *testing.T,
) {
	moveCount := 0

	gridInstance := domainGrid.NewGridWithLabels(
		"ABCD",
		"",
		"",
		image.Rect(0, 0, 100, 100),
		zap.NewNop(),
	)

	handler := newHandlerWithState(handlerState{
		config: &config.Config{
			Grid: config.GridConfig{
				Enabled:    true,
				Characters: "ABCD",
				Hotkeys:    map[string]config.StringOrStringArray{},
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
		grid: &components.GridComponent{
			Context: &gridcomponent.Context{},
		},
		renderer:     render.NewOverlayRenderer(&overlaypkg.NoOpManager{}, nil),
		screenBounds: image.Rect(0, 0, 100, 100),
	})

	handler.initializeGridManager(gridInstance)
	handler.grid.Router = domainGrid.NewRouter(handler.grid.Manager, zap.NewNop())
	handler.grid.Context.SetCursorFollowSelection(false)

	handler.handleGridModeKey("A")
	handler.handleGridModeKey("A")

	if moveCount != 0 {
		t.Fatalf(
			"handleGridModeKey() moved cursor %d times while entering subgrid, want 0",
			moveCount,
		)
	}

	if _, ok := handler.grid.Context.SelectionPoint(); !ok {
		t.Fatal("expected subgrid entry selection point to be stored")
	}
}

// TestApplyGridFlags_TellsAbsentOnExitFromEmptyOne pins the --on-exit contract
// for grid, where nil and empty are different values rather than two spellings
// of nothing. A repeat re-activation carries no --on-exit and must keep the
// steps the mode was activated with; a command that gave --on-exit no steps is
// asking for none to run.
func TestApplyGridFlags_TellsAbsentOnExitFromEmptyOne(t *testing.T) {
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
			ctx := &gridcomponent.Context{}
			ctx.SetOnExit(stored)

			applyGridFlags(ctx, modecmd.Activation{OnExit: testCase.onExit}, testCase.isRefresh)

			if len(ctx.OnExit()) != testCase.want {
				t.Errorf("OnExit = %v, want %d step(s)", ctx.OnExit(), testCase.want)
			}
		})
	}
}
