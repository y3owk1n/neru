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
	"github.com/y3owk1n/neru/internal/domain/recursivegrid"
	"github.com/y3owk1n/neru/internal/domain/state"
	"github.com/y3owk1n/neru/internal/ports"
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

// TestRecursiveGridFrame_NonSquareGridKeepsRowsAndColumnsApart pins the one
// conversion left on the recursive-grid draw path: the mode layer is where the
// manager's separate row and column counts are paired up into the
// domain.GridDimensions the port carries all the way to the division (#1313).
//
// Everything downstream now takes that value whole, so writing the counts under
// each other's names here would transpose every recursive-grid cell on every
// backend and nothing else would notice. The grid is deliberately non-square,
// and its next depth a different non-square shape, so a swap cannot survive
// either conversion — and the cells the frame's dimensions divide into are
// checked too, because a transposition is only a bug for what it does to the
// screen.
func TestRecursiveGridFrame_NonSquareGridKeepsRowsAndColumnsApart(t *testing.T) {
	overlay := &portmocks.MockOverlayPort{}

	handler := newHandlerWithState(handlerState{
		config: &config.Config{
			RecursiveGrid: config.RecursiveGridConfig{
				Enabled:       true,
				GridCols:      4,
				GridRows:      2,
				Keys:          "uiopjkl;",
				MinSizeWidth:  5,
				MinSizeHeight: 5,
				MaxDepth:      5,
				Layers: []config.RecursiveGridLayerConfig{
					{Depth: 1, GridCols: 3, GridRows: 2, Keys: "asdfgh"},
				},
				Hotkeys: map[string]config.StringOrStringArray{},
				UI:      config.RecursiveGridUI{},
			},
		},
		overlayPort: overlay,
		recursiveGrid: &components.RecursiveGridComponent{
			Context: &componentrecursivegrid.Context{},
		},
		screenBounds: image.Rect(0, 0, 400, 200),
	})

	handler.initializeRecursiveGridManager(image.Rect(0, 0, 400, 200))
	handler.updateRecursiveGridOverlay()

	frames := overlay.Frames()
	if len(frames) != 1 {
		t.Fatalf("overlay received %d frame(s), want 1", len(frames))
	}

	frame, isRecursiveGrid := frames[0].(ports.RecursiveGridFrame)
	if !isRecursiveGrid {
		t.Fatalf("overlay received a %T, want ports.RecursiveGridFrame", frames[0])
	}

	if got, want := frame.Layout.Dimensions, (domain.GridDimensions{Rows: 2, Cols: 4}); got != want {
		t.Errorf("Layout.Dimensions = %+v, want %+v", got, want)
	}

	if got, want := frame.NextLayout.Dimensions, (domain.GridDimensions{Rows: 2, Cols: 3}); got != want {
		t.Errorf("NextLayout.Dimensions = %+v, want %+v", got, want)
	}

	cells := recursivegrid.ComputeGridCells(frame.Bounds, frame.Layout.Dimensions)
	if len(cells) != 8 {
		t.Fatalf("the frame's dimensions divide into %d cells, want 8", len(cells))
	}

	if got, want := cells[0], image.Rect(0, 0, 100, 100); got != want {
		t.Errorf("first drawn cell = %v, want %v", got, want)
	}
}

// TestRecursiveGridFrame_DegenerateShapeReachesEveryBackendAsTheDefault is what
// makes "every backend draws the same thing" true for a shape that cannot
// narrow anything (#1345).
//
// Only macOS corrects such a shape in its draw. Linux and Windows guard the
// division and nothing more: a side of zero or less stops their draw entirely,
// and a 1x1 divides into one cell over the whole region that no key can narrow.
// That difference is unreachable, and this is where it is held unreachable: the
// manager replaces an unusable shape before anything is drawn, so the frame
// every backend is handed already carries a usable one and the key mapping that
// fits it. If the fallback ever moved out of the domain and into the macOS draw
// alone, this fails rather than the two other platforms quietly drawing
// something a person cannot navigate.
func TestRecursiveGridFrame_DegenerateShapeReachesEveryBackendAsTheDefault(t *testing.T) {
	testCases := []struct {
		name string
		cols int
		rows int
		keys string
	}{
		{name: "one cell cannot narrow", cols: 1, rows: 1, keys: "u"},
		{name: "no columns", cols: 0, rows: 3, keys: "rtyfghvbn"},
		{name: "no rows", cols: 3, rows: 0, keys: "rtyfghvbn"},
		{name: "nothing configured at all", cols: 0, rows: 0, keys: ""},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			overlay := &portmocks.MockOverlayPort{}

			handler := newHandlerWithState(handlerState{
				config: &config.Config{
					RecursiveGrid: config.RecursiveGridConfig{
						Enabled:       true,
						GridCols:      testCase.cols,
						GridRows:      testCase.rows,
						Keys:          testCase.keys,
						MinSizeWidth:  5,
						MinSizeHeight: 5,
						MaxDepth:      5,
						Hotkeys:       map[string]config.StringOrStringArray{},
						UI:            config.RecursiveGridUI{},
					},
				},
				overlayPort: overlay,
				recursiveGrid: &components.RecursiveGridComponent{
					Context: &componentrecursivegrid.Context{},
				},
				screenBounds: image.Rect(0, 0, 300, 300),
			})

			handler.initializeRecursiveGridManager(image.Rect(0, 0, 300, 300))
			handler.updateRecursiveGridOverlay()

			frames := overlay.Frames()
			if len(frames) != 1 {
				t.Fatalf("overlay received %d frame(s), want 1", len(frames))
			}

			frame, isRecursiveGrid := frames[0].(ports.RecursiveGridFrame)
			if !isRecursiveGrid {
				t.Fatalf("overlay received a %T, want ports.RecursiveGridFrame", frames[0])
			}

			if got, want := frame.Layout.Dimensions, recursivegrid.DefaultDimensions(); got != want {
				t.Errorf("Layout.Dimensions = %+v, want %+v", got, want)
			}

			if got, want := frame.Layout.Keys, recursivegrid.DefaultKeys; got != want {
				t.Errorf("Layout.Keys = %q, want %q", got, want)
			}

			// The division is what a backend actually paints, and every
			// backend runs this same one over the frame's dimensions. A
			// 300x300 screen in the default 3x3 gives nine 100x100 cells.
			cells := recursivegrid.ComputeGridCells(frame.Bounds, frame.Layout.Dimensions)
			if len(cells) != 9 {
				t.Fatalf("the frame's dimensions divide into %d cells, want 9", len(cells))
			}

			if got, want := cells[0], image.Rect(0, 0, 100, 100); got != want {
				t.Errorf("first drawn cell = %v, want %v", got, want)
			}

			if got, want := cells[8], image.Rect(200, 200, 300, 300); got != want {
				t.Errorf("last drawn cell = %v, want %v", got, want)
			}
		})
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
