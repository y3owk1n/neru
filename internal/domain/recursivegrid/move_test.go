package recursivegrid_test

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/recursivegrid"
)

// newMoveGrid builds a 900x900 grid divided 3x3 at every depth, so depth 1
// cells are 300px and depth 2 cells are 100px.
func newMoveGrid() *recursivegrid.RecursiveGrid {
	return recursivegrid.NewRecursiveGridWithLayers(
		image.Rect(0, 0, 900, 900),
		1, 1, 10, domain.GridDimensions{Rows: 3, Cols: 3},
		nil,
	)
}

// descend selects the given cells in order, failing the test if the grid
// bottoms out early.
func descend(t *testing.T, grid *recursivegrid.RecursiveGrid, cells ...recursivegrid.Cell) {
	t.Helper()

	for _, cell := range cells {
		_, complete := grid.SelectCell(cell)
		require.False(t, complete, "grid bottomed out before the test could set up")
	}
}

func TestRecursiveGrid_MoveDirection_WithinParent(t *testing.T) {
	grid := newMoveGrid()
	// Depth 1 top-left cell, then its own top-left: (0,0)-(100,100).
	descend(t, grid, 0, 0)

	center, moved := grid.MoveDirection(domain.DirectionRight, 1)

	assert.True(t, moved, "moving right from the leftmost cell should succeed")
	assert.Equal(t, image.Rect(100, 0, 200, 100), grid.CurrentBounds())
	assert.Equal(t, image.Point{X: 150, Y: 50}, center)
	assert.Equal(t, 2, grid.CurrentDepth(), "depth must not change")
}

func TestRecursiveGrid_MoveDirection_CrossesIntoNeighbouringParent(t *testing.T) {
	grid := newMoveGrid()
	// Depth 1 top-left cell, then its top-right: (200,0)-(300,100).
	// The next cell to the right lives under a different depth-1 parent.
	descend(t, grid, 0, 2)
	require.Equal(t, image.Rect(200, 0, 300, 100), grid.CurrentBounds())

	center, moved := grid.MoveDirection(domain.DirectionRight, 1)

	assert.True(t, moved, "crossing a parent boundary should succeed")
	assert.Equal(t, image.Rect(300, 0, 400, 100), grid.CurrentBounds())
	assert.Equal(t, image.Point{X: 350, Y: 50}, center)
	assert.Equal(t, 2, grid.CurrentDepth())
}

func TestRecursiveGrid_MoveDirection_RebuildsAncestors(t *testing.T) {
	grid := newMoveGrid()
	descend(t, grid, 0, 2)

	_, moved := grid.MoveDirection(domain.DirectionRight, 1)
	require.True(t, moved)

	// The ancestor chain must follow the selection into the new parent,
	// otherwise backtracking would surface a region that no longer contains
	// the current bounds.
	require.True(t, grid.Backtrack(), "history should still be intact after a move")
	assert.Equal(t, image.Rect(300, 0, 600, 300), grid.CurrentBounds())
	assert.Equal(t, 1, grid.CurrentDepth())

	require.True(t, grid.Backtrack())
	assert.Equal(t, image.Rect(0, 0, 900, 900), grid.CurrentBounds())
	assert.Equal(t, 0, grid.CurrentDepth())
}

func TestRecursiveGrid_MoveDirection_StopsAtScreenEdge(t *testing.T) {
	grid := newMoveGrid()
	descend(t, grid, 0, 0)

	before := grid.CurrentBounds()

	center, moved := grid.MoveDirection(domain.DirectionLeft, 1)

	assert.False(t, moved, "moving left off the screen should be refused")
	assert.Equal(t, before, grid.CurrentBounds(), "a refused move must not change state")
	assert.Equal(t, image.Point{X: 50, Y: 50}, center)
}

func TestRecursiveGrid_MoveDirection_NoOpAtDepthZero(t *testing.T) {
	grid := newMoveGrid()

	// Nothing is selected yet at depth 0: the selection is the whole screen,
	// so a step of one screen width always lands outside.
	for _, dir := range []domain.Direction{
		domain.DirectionLeft,
		domain.DirectionRight,
		domain.DirectionUp,
		domain.DirectionDown,
	} {
		_, moved := grid.MoveDirection(dir, 1)

		assert.Falsef(t, moved, "move %s at depth 0 should be a no-op", dir)
		assert.Equal(t, image.Rect(0, 0, 900, 900), grid.CurrentBounds())
	}
}

func TestRecursiveGrid_MoveDirection_Count(t *testing.T) {
	tests := []struct {
		name      string
		direction domain.Direction
		count     int
		want      image.Rectangle
		wantMoved bool
	}{
		{
			name:      "two cells right",
			direction: domain.DirectionRight,
			count:     2,
			want:      image.Rect(200, 0, 300, 100),
			wantMoved: true,
		},
		{
			name:      "four cells down crosses a parent",
			direction: domain.DirectionDown,
			count:     4,
			want:      image.Rect(0, 400, 100, 500),
			wantMoved: true,
		},
		{
			name:      "count below one is treated as one",
			direction: domain.DirectionRight,
			count:     0,
			want:      image.Rect(100, 0, 200, 100),
			wantMoved: true,
		},
		{
			name:      "a move that cannot take its first step reports no movement",
			direction: domain.DirectionUp,
			count:     3,
			want:      image.Rect(0, 0, 100, 100),
			wantMoved: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			grid := newMoveGrid()
			descend(t, grid, 0, 0)

			_, moved := grid.MoveDirection(testCase.direction, testCase.count)

			assert.Equal(t, testCase.wantMoved, moved)
			assert.Equal(t, testCase.want, grid.CurrentBounds())
			assert.Equal(t, 2, grid.CurrentDepth())
		})
	}
}

func TestRecursiveGrid_MoveDirection_PartialCountStopsAtEdge(t *testing.T) {
	grid := newMoveGrid()
	descend(t, grid, 0, 0)

	// The 900px row holds nine 100px cells, so only eight of the twenty
	// requested steps fit. The rest are dropped at the edge.
	_, moved := grid.MoveDirection(domain.DirectionRight, 20)

	assert.True(t, moved, "a partially applied move still counts as movement")
	assert.Equal(t, image.Rect(800, 0, 900, 100), grid.CurrentBounds(),
		"movement should stop on the last cell inside the screen")
}

func TestRecursiveGrid_MoveDirection_PerDepthLayouts(t *testing.T) {
	// Depth 1 splits 2x2 rather than the default 3x3, so a depth-2 cell is
	// 150px wide: movement has to read the layout of each depth it walks.
	grid := recursivegrid.NewRecursiveGridWithLayers(
		image.Rect(0, 0, 900, 900),
		1, 1, 10, domain.GridDimensions{Rows: 3, Cols: 3},
		map[int]recursivegrid.DepthLayout{1: {GridCols: 2, GridRows: 2}},
	)

	descend(t, grid, 0, 0)
	require.Equal(t, image.Rect(0, 0, 150, 150), grid.CurrentBounds())

	_, moved := grid.MoveDirection(domain.DirectionRight, 1)

	assert.True(t, moved)
	assert.Equal(t, image.Rect(150, 0, 300, 150), grid.CurrentBounds())
	assert.Equal(t, 2, grid.CurrentDepth())
}

func TestRecursiveGrid_MoveDirection_MovesFinalCellAfterBottomingOut(t *testing.T) {
	// minSize 200 lets depth 0 divide (300px cells) but not depth 1 (100px),
	// so the depth-1 selection is a final cell that never becomes the bounds.
	grid := recursivegrid.NewRecursiveGridWithLayers(
		image.Rect(0, 0, 900, 900),
		200, 200, 10, domain.GridDimensions{Rows: 3, Cols: 3},
		nil,
	)

	_, complete := grid.SelectCell(0)
	require.False(t, complete)

	// Cell 4 is the middle of the depth-1 grid: (100,100)-(200,200).
	center, complete := grid.SelectCell(4)
	require.True(t, complete, "the grid should have bottomed out at depth 1")
	require.Equal(t, image.Point{X: 150, Y: 150}, center)
	require.True(t, grid.HasFinalCell())
	require.Equal(t, image.Rect(100, 100, 200, 200), grid.EffectiveBounds())

	// The move must step by the final cell, not by the much larger bounds.
	center, moved := grid.MoveDirection(domain.DirectionRight, 1)

	assert.True(t, moved)
	assert.Equal(t, image.Point{X: 250, Y: 150}, center)
	assert.Equal(t, image.Rect(200, 100, 300, 200), grid.EffectiveBounds())
	assert.Equal(t, 1, grid.CurrentDepth())
}

func TestRecursiveGrid_MoveDirection_FinalCellCrossesIntoNeighbouringParent(t *testing.T) {
	grid := recursivegrid.NewRecursiveGridWithLayers(
		image.Rect(0, 0, 900, 900),
		200, 200, 10, domain.GridDimensions{Rows: 3, Cols: 3},
		nil,
	)

	_, complete := grid.SelectCell(0)
	require.False(t, complete)

	// Cell 5 is the middle-right of the depth-1 grid: (200,100)-(300,200),
	// flush against the right edge of its parent.
	_, complete = grid.SelectCell(5)
	require.True(t, complete)

	center, moved := grid.MoveDirection(domain.DirectionRight, 1)

	assert.True(t, moved)
	assert.Equal(t, image.Rect(300, 0, 600, 300), grid.CurrentBounds(),
		"the parent should have moved with the final cell")
	assert.Equal(t, image.Rect(300, 100, 400, 200), grid.EffectiveBounds())
	assert.Equal(t, image.Point{X: 350, Y: 150}, center)
}

func TestRecursiveGrid_MoveDirection_RefusesWhenTargetDepthIsUnreachable(t *testing.T) {
	// A 106px row split into three gives cells of 36, 35 and 35 pixels. With a
	// minimum of 12, only the 36px parent can divide again (36/3 == 12), so a
	// depth-2 selection cannot exist under the 35px neighbors.
	grid := recursivegrid.NewRecursiveGridWithLayers(
		image.Rect(0, 0, 106, 100),
		12, 1, 10, domain.GridDimensions{Rows: 1, Cols: 3},
		nil,
	)

	descend(t, grid, 0, 2)
	require.Equal(t, image.Rect(24, 0, 36, 100), grid.CurrentBounds())

	beforeBounds := grid.CurrentBounds()
	beforeDepth := grid.CurrentDepth()

	_, moved := grid.MoveDirection(domain.DirectionRight, 1)

	assert.False(t, moved, "the move should be refused rather than land at a shallower depth")
	assert.Equal(t, beforeBounds, grid.CurrentBounds(), "state must be rolled back intact")
	assert.Equal(t, beforeDepth, grid.CurrentDepth())

	// History survived the rolled-back re-walk.
	require.True(t, grid.Backtrack())
	assert.Equal(t, image.Rect(0, 0, 36, 100), grid.CurrentBounds())
}

func TestManager_MoveDirection_NotifiesOnlyWhenSomethingMoved(t *testing.T) {
	updates := 0
	manager := recursivegrid.NewManagerWithLayers(
		image.Rect(0, 0, 900, 900),
		"rtyfghvbn",
		1, 1, 10, domain.GridDimensions{Rows: 3, Cols: 3},
		nil, nil,
		recursivegrid.SelectionCallbacks{
			OnUpdate:   func(image.Point) { updates++ },
			OnComplete: func(image.Point) {},
		},
		zap.NewNop(),
	)

	// Drill to the top-left 100x100 cell; each selection fires its own update.
	manager.HandleInput("r")
	manager.HandleInput("r")

	updatesAfterSetup := updates

	center, moved := manager.MoveDirection(domain.DirectionRight, 1)

	require.True(t, moved)
	assert.Equal(t, image.Point{X: 150, Y: 50}, center)
	assert.Equal(t, updatesAfterSetup+1, updates, "a move should refresh the overlay")

	// Against the left edge nothing moves, so nothing should be redrawn.
	_, moved = manager.MoveDirection(domain.DirectionUp, 1)

	assert.False(t, moved)
	assert.Equal(t, updatesAfterSetup+1, updates, "a refused move should not redraw")
}

func TestRecursiveGrid_Backtrack_ClearsFinalCell(t *testing.T) {
	grid := recursivegrid.NewRecursiveGridWithLayers(
		image.Rect(0, 0, 900, 900),
		200, 200, 10, domain.GridDimensions{Rows: 3, Cols: 3},
		nil,
	)

	_, complete := grid.SelectCell(0)
	require.False(t, complete)

	_, complete = grid.SelectCell(4)
	require.True(t, complete)
	require.True(t, grid.HasFinalCell())

	require.True(t, grid.Backtrack())

	assert.False(t, grid.HasFinalCell(), "backtracking past a final cell should drop it")
	assert.Equal(t, image.Rect(0, 0, 900, 900), grid.EffectiveBounds())
}

func TestRecursiveGrid_Reset_ClearsFinalCell(t *testing.T) {
	grid := recursivegrid.NewRecursiveGridWithLayers(
		image.Rect(0, 0, 900, 900),
		200, 200, 10, domain.GridDimensions{Rows: 3, Cols: 3},
		nil,
	)

	_, complete := grid.SelectCell(0)
	require.False(t, complete)

	_, complete = grid.SelectCell(4)
	require.True(t, complete)

	grid.Reset()

	assert.False(t, grid.HasFinalCell())
	assert.Equal(t, image.Rect(0, 0, 900, 900), grid.EffectiveBounds())
	assert.Equal(t, 0, grid.CurrentDepth())
}

func TestRecursiveGrid_MoveDirection_SinglePixelCells(t *testing.T) {
	// A 9px row split 3x3 twice bottoms out at one-pixel cells. rectCenter
	// rounds up, so the center of a 1px cell sits outside it: probing from
	// there used to skip a cell and drift diagonally.
	grid := recursivegrid.NewRecursiveGridWithLayers(
		image.Rect(0, 0, 9, 9),
		1, 1, 10, domain.GridDimensions{Rows: 3, Cols: 3},
		nil,
	)

	descend(t, grid, 0, 0)
	require.Equal(t, image.Rect(0, 0, 1, 1), grid.CurrentBounds())

	want := []image.Rectangle{
		image.Rect(1, 0, 2, 1),
		image.Rect(2, 0, 3, 1),
		// Crossing into the neighboring parent at x=3.
		image.Rect(3, 0, 4, 1),
		image.Rect(4, 0, 5, 1),
	}

	for step, wantBounds := range want {
		_, moved := grid.MoveDirection(domain.DirectionRight, 1)

		require.Truef(t, moved, "step %d should have moved", step)
		assert.Equalf(t, wantBounds, grid.CurrentBounds(),
			"step %d moved by exactly one cell with no vertical drift", step)
		assert.Equal(t, 2, grid.CurrentDepth())
	}
}

func TestRecursiveGrid_MoveDirection_AfterScreenChange(t *testing.T) {
	// A monitor switch or resolution change remaps the grid proportionally.
	// The final cell has to survive that and still move by one cell on the
	// new geometry.
	grid := recursivegrid.NewRecursiveGridWithLayers(
		image.Rect(0, 0, 900, 900),
		200, 200, 10, domain.GridDimensions{Rows: 3, Cols: 3},
		nil,
	)

	_, complete := grid.SelectCell(0)
	require.False(t, complete)

	_, complete = grid.SelectCell(4)
	require.True(t, complete)
	require.Equal(t, image.Rect(100, 100, 200, 200), grid.EffectiveBounds())

	grid.RemapToNewBounds(image.Rect(0, 0, 1800, 1800))

	require.True(t, grid.HasFinalCell(), "a screen change should not drop the selection")
	assert.Equal(t, image.Rect(200, 200, 400, 400), grid.EffectiveBounds(),
		"the final cell should scale with the new bounds")

	_, moved := grid.MoveDirection(domain.DirectionRight, 1)

	assert.True(t, moved)
	assert.Equal(t, image.Rect(400, 200, 600, 400), grid.EffectiveBounds(),
		"movement should step by the remapped cell size")
}

// TestRecursiveGrid_SelectionCenter_StaysInsideOnePixelCells pins the invariant
// every cursor target depends on: the reported center of a selection must lie
// inside that selection. Rounding to nearest breaks it for a one-pixel cell,
// which puts the cursor — and any action targeting it — on the cell next door.
func TestRecursiveGrid_SelectionCenter_StaysInsideOnePixelCells(t *testing.T) {
	t.Run("after a directional move", func(t *testing.T) {
		grid := recursivegrid.NewRecursiveGridWithLayers(
			image.Rect(0, 0, 9, 9),
			1, 1, 10, domain.GridDimensions{Rows: 3, Cols: 3},
			nil,
		)

		descend(t, grid, 0, 0)
		require.Equal(t, image.Rect(0, 0, 1, 1), grid.CurrentBounds())

		_, moved := grid.MoveDirection(domain.DirectionRight, 3)
		require.True(t, moved)

		bounds := grid.EffectiveBounds()
		center := grid.SelectionCenter()

		assert.Truef(t, center.In(bounds),
			"selection center %v is outside the selected cell %v", center, bounds)
	})

	t.Run("after selecting a final cell", func(t *testing.T) {
		// A minimum size of 2 stops subdivision while the cells drawn at that
		// depth are still one pixel, so the final cell is one pixel.
		grid := recursivegrid.NewRecursiveGridWithLayers(
			image.Rect(0, 0, 9, 9),
			2, 2, 10, domain.GridDimensions{Rows: 3, Cols: 3},
			nil,
		)

		_, complete := grid.SelectCell(0)
		require.False(t, complete)

		center, complete := grid.SelectCell(4)
		require.True(t, complete)

		bounds := grid.CellBounds(4)
		require.Equal(t, image.Rect(1, 1, 2, 2), bounds)

		assert.Truef(t, center.In(bounds),
			"selected point %v is outside the final cell %v", center, bounds)
		assert.Equal(t, center, grid.SelectionCenter(),
			"SelectionCenter should agree with the point SelectCell returned")
	})

	t.Run("every cell of a one-pixel grid", func(t *testing.T) {
		grid := recursivegrid.NewRecursiveGridWithLayers(
			image.Rect(0, 0, 3, 3),
			1, 1, 10, domain.GridDimensions{Rows: 3, Cols: 3},
			nil,
		)

		for cell := range recursivegrid.Cell(9) {
			bounds := grid.CellBounds(cell)
			center := grid.CellCenter(cell)

			assert.Truef(t, center.In(bounds),
				"cell %d center %v is outside its bounds %v", cell, center, bounds)
		}
	})
}
