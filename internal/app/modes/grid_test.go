//nolint:testpackage // Tests private grid handler behavior.
package modes

import (
	"context"
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	gridcomponent "github.com/y3owk1n/neru/internal/app/components/grid"
	hintscomponent "github.com/y3owk1n/neru/internal/app/components/hints"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	portmocks "github.com/y3owk1n/neru/internal/core/ports/mocks"
	"github.com/y3owk1n/neru/internal/ui"
	overlaypkg "github.com/y3owk1n/neru/internal/ui/overlay"
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

	handler := &Handler{
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
			&portmocks.SystemMock{
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
	}

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

	handler := &Handler{
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
			&portmocks.SystemMock{
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
		renderer: ui.NewOverlayRenderer(
			&overlaypkg.NoOpManager{},
			hintscomponent.StyleMode{},
			gridcomponent.Style{},
			recursivegridcomponent.Style{},
		),
		screenBounds: image.Rect(0, 0, 100, 100),
	}

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
