//nolint:testpackage // Tests private recursive-grid handler behavior.
package modes

import (
	"context"
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	gridcomponent "github.com/y3owk1n/neru/internal/app/components/grid"
	hintscomponent "github.com/y3owk1n/neru/internal/app/components/hints"
	componentrecursivegrid "github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	portmocks "github.com/y3owk1n/neru/internal/core/ports/mocks"
	"github.com/y3owk1n/neru/internal/ui"
	overlaypkg "github.com/y3owk1n/neru/internal/ui/overlay"
)

type recordingOverlayManager struct {
	overlaypkg.NoOpManager
	lastHideUnmatched       bool
	setHideUnmatchedInvoked int
}

func (m *recordingOverlayManager) SetHideUnmatched(hide bool) {
	m.lastHideUnmatched = hide
	m.setHideUnmatchedInvoked++
}

func TestHandleRecursiveGridKey_CompleteSelectionDoesNotMoveWhenCursorFollowSelectionDisabled(
	t *testing.T,
) {
	moveCount := 0

	handler := &Handler{
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
			&portmocks.SystemMock{
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
	}

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

	handler := &Handler{
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
			&portmocks.SystemMock{
				MoveCursorToPointFunc: func(_ context.Context, _ image.Point, _ bool) error {
					moveCount++

					return nil
				},
			},
			zap.NewNop(),
		),
		renderer: ui.NewOverlayRenderer(
			&overlaypkg.NoOpManager{},
			hintscomponent.StyleMode{},
			gridcomponent.Style{},
			componentrecursivegrid.Style{},
		),
		recursiveGrid: &components.RecursiveGridComponent{
			Context: &componentrecursivegrid.Context{},
		},
		screenBounds: image.Rect(0, 0, 100, 100),
	}

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

func TestCleanupGridModeResetsHideUnmatched(t *testing.T) {
	overlayManager := &recordingOverlayManager{}

	handler := &Handler{
		overlayManager: overlayManager,
		grid: &components.GridComponent{
			Context: &gridcomponent.Context{},
		},
		logger: zap.NewNop(),
		renderer: ui.NewOverlayRenderer(
			overlayManager,
			hintscomponent.StyleMode{},
			gridcomponent.Style{},
			componentrecursivegrid.Style{},
		),
	}

	handler.cleanupGridMode()

	if overlayManager.setHideUnmatchedInvoked == 0 {
		t.Fatal("expected grid cleanup to reset hide unmatched")
	}

	if overlayManager.lastHideUnmatched {
		t.Fatal("expected grid cleanup to disable hide unmatched")
	}
}

func TestResetCurrentMode_RecursiveGridTrainingReturnsToTopLevel(t *testing.T) {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeRecursiveGrid)

	handler := &Handler{
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
				Training: config.RecursiveGridTrainingConfig{
					Enabled:        true,
					HitsToHide:     3,
					PenaltyOnError: 1,
				},
				Hotkeys: map[string]config.StringOrStringArray{},
				UI:      config.RecursiveGridUI{},
			},
		},
		logger: zap.NewNop(),
		renderer: ui.NewOverlayRenderer(
			&overlaypkg.NoOpManager{},
			hintscomponent.StyleMode{},
			gridcomponent.Style{},
			componentrecursivegrid.Style{},
		),
		recursiveGrid: &components.RecursiveGridComponent{
			Context: &componentrecursivegrid.Context{},
		},
		screenBounds: image.Rect(0, 0, 100, 100),
	}

	handler.initializeRecursiveGridManager(image.Rect(0, 0, 100, 100))
	handler.recursiveGrid.Training = componentrecursivegrid.NewTrainingSession("uijk", "uijk", true, 3, 1)
	targetKey := string([]rune("uijk")[handler.recursiveGrid.Training.TargetIndex()])

	if result := handler.recursiveGrid.Training.HandleKey(targetKey); result != componentrecursivegrid.TrainingResultAdvanceDepth {
		t.Fatalf("first training step result = %v, want %v", result, componentrecursivegrid.TrainingResultAdvanceDepth)
	}

	handler.recursiveGrid.Manager.HandleInput(targetKey)

	if got := handler.recursiveGrid.Manager.CurrentDepth(); got != 1 {
		t.Fatalf("manager depth before reset = %d, want 1", got)
	}

	handler.ResetCurrentMode()

	if got := handler.recursiveGrid.Manager.CurrentDepth(); got != 0 {
		t.Fatalf("manager depth after reset = %d, want 0", got)
	}

	if handler.recursiveGrid.Training == nil || !handler.recursiveGrid.Training.Active() {
		t.Fatal("training session should remain active after reset")
	}

	if handler.recursiveGrid.Training.InSecondDepth() {
		t.Fatal("training session should return to top-level after reset")
	}
}
