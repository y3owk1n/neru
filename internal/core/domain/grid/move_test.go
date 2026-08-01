package grid_test

import (
	"image"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/grid"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
)

// moveHarness wires a manager to a grid and records the cells the subgrid
// callback is asked to draw.
type moveHarness struct {
	grid    *grid.Grid
	manager *grid.Manager
	shown   []*grid.Cell
}

func newMoveHarness(t *testing.T) *moveHarness {
	t.Helper()

	log := logger.Get()
	harness := &moveHarness{
		grid: grid.NewGrid("abcdefghijklmnopqrstuvwxyz", image.Rect(0, 0, 1000, 800), log),
	}

	harness.manager = grid.NewManager(
		harness.grid,
		domain.SubgridRows,
		domain.SubgridCols,
		"asdfghjkl",
		func(bool) {},
		func(cell *grid.Cell) { harness.shown = append(harness.shown, cell) },
		log,
	)

	return harness
}

// openSubgrid types the coordinate of cell so the manager enters its subgrid.
func (h *moveHarness) openSubgrid(t *testing.T, cell *grid.Cell) {
	t.Helper()

	for _, char := range cell.Coordinate() {
		h.manager.HandleInput(string(char))
	}

	if len(h.shown) == 0 {
		t.Fatalf("typing %q did not open a subgrid", cell.Coordinate())
	}

	if got := h.shown[len(h.shown)-1]; got.Coordinate() != cell.Coordinate() {
		t.Fatalf("opened subgrid for %q, want %q", got.Coordinate(), cell.Coordinate())
	}
}

// cellRightOf returns the cell one cell-width to the right of cell, or nil.
func (h *moveHarness) cellRightOf(cell *grid.Cell) *grid.Cell {
	return h.grid.CellForPoint(image.Point{
		X: cell.Center().X + cell.Bounds().Dx(),
		Y: cell.Center().Y,
	})
}

// anInteriorCell returns a cell that has a neighbor to its right.
func (h *moveHarness) anInteriorCell(t *testing.T) *grid.Cell {
	t.Helper()

	for _, cell := range h.grid.AllCells() {
		if h.cellRightOf(cell) != nil {
			return cell
		}
	}

	t.Fatal("grid has no cell with a right neighbor")

	return nil
}

// rightmostCell returns a cell on the right edge of the grid.
func (h *moveHarness) rightmostCell(t *testing.T) *grid.Cell {
	t.Helper()

	var rightmost *grid.Cell

	for _, cell := range h.grid.AllCells() {
		if rightmost == nil || cell.Bounds().Max.X > rightmost.Bounds().Max.X {
			rightmost = cell
		}
	}

	if rightmost == nil {
		t.Fatal("grid has no cells")
	}

	return rightmost
}

func TestManager_MoveDirection_MovesSubgridToNeighbouringCell(t *testing.T) {
	harness := newMoveHarness(t)
	start := harness.anInteriorCell(t)
	want := harness.cellRightOf(start)

	harness.openSubgrid(t, start)
	shownBefore := len(harness.shown)

	center, moved := harness.manager.MoveDirection(domain.DirectionRight, 1)

	if !moved {
		t.Fatal("MoveDirection(right) reported no movement")
	}

	if center != want.Center() {
		t.Errorf("MoveDirection(right) center = %v, want %v", center, want.Center())
	}

	if len(harness.shown) != shownBefore+1 {
		t.Fatalf("expected one subgrid redraw, got %d", len(harness.shown)-shownBefore)
	}

	if got := harness.shown[len(harness.shown)-1]; got.Coordinate() != want.Coordinate() {
		t.Errorf("subgrid redrawn for %q, want %q", got.Coordinate(), want.Coordinate())
	}
}

func TestManager_MoveDirection_IgnoredWithoutAnOpenSubgrid(t *testing.T) {
	harness := newMoveHarness(t)

	// Nothing typed yet: no cell is selected, so there is nothing to move.
	if _, moved := harness.manager.MoveDirection(domain.DirectionRight, 1); moved {
		t.Error("MoveDirection with no selection should report no movement")
	}

	// A partial coordinate is still not a selection.
	harness.manager.HandleInput(string(harness.anInteriorCell(t).Coordinate()[0]))

	if _, moved := harness.manager.MoveDirection(domain.DirectionRight, 1); moved {
		t.Error("MoveDirection on a partial coordinate should report no movement")
	}

	if len(harness.shown) != 0 {
		t.Errorf("expected no subgrid draws, got %d", len(harness.shown))
	}
}

func TestManager_MoveDirection_StopsAtGridEdge(t *testing.T) {
	harness := newMoveHarness(t)
	edge := harness.rightmostCell(t)

	harness.openSubgrid(t, edge)
	shownBefore := len(harness.shown)

	center, moved := harness.manager.MoveDirection(domain.DirectionRight, 1)

	if moved {
		t.Error("MoveDirection past the right edge should report no movement")
	}

	if center != edge.Center() {
		t.Errorf("center = %v, want the unchanged %v", center, edge.Center())
	}

	if len(harness.shown) != shownBefore {
		t.Error("a refused move should not redraw the subgrid")
	}
}

func TestManager_MoveDirection_KeepsBackspaceInputInSync(t *testing.T) {
	harness := newMoveHarness(t)
	start := harness.anInteriorCell(t)
	want := harness.cellRightOf(start)

	harness.openSubgrid(t, start)

	if _, moved := harness.manager.MoveDirection(domain.DirectionRight, 1); !moved {
		t.Fatal("MoveDirection(right) reported no movement")
	}

	// Backspacing out of the subgrid restores the main-grid coordinate, which
	// has to describe the cell the move landed on rather than the one first
	// typed — otherwise the input no longer matches the highlighted cell.
	harness.manager.HandleBackspace()

	wantInput := want.Coordinate()[:len(want.Coordinate())-1]
	if got := harness.manager.CurrentInput(); got != wantInput {
		t.Errorf("input after backspace = %q, want %q", got, wantInput)
	}

	if !strings.HasPrefix(want.Coordinate(), harness.manager.CurrentInput()) {
		t.Errorf(
			"restored input %q is not a prefix of the selected cell %q",
			harness.manager.CurrentInput(),
			want.Coordinate(),
		)
	}
}

func TestManager_MoveDirection_Count(t *testing.T) {
	harness := newMoveHarness(t)

	start := harness.anInteriorCell(t)

	second := harness.cellRightOf(start)
	if second == nil {
		t.Fatal("no first neighbor")
	}

	want := harness.cellRightOf(second)
	if want == nil {
		t.Skip("grid is too narrow for a two-cell move")
	}

	harness.openSubgrid(t, start)

	center, moved := harness.manager.MoveDirection(domain.DirectionRight, 2)

	if !moved {
		t.Fatal("MoveDirection(right, 2) reported no movement")
	}

	if center != want.Center() {
		t.Errorf("center = %v, want %v", center, want.Center())
	}
}

func TestGrid_CellForPoint(t *testing.T) {
	testGrid := grid.NewGrid(
		"abcdefghijklmnopqrstuvwxyz",
		image.Rect(0, 0, 1000, 800),
		logger.Get(),
	)

	for _, cell := range testGrid.AllCells() {
		if got := testGrid.CellForPoint(cell.Center()); got != cell {
			t.Fatalf(
				"CellForPoint(center of %q) returned %v, want the cell itself",
				cell.Coordinate(),
				got,
			)
		}
	}

	if got := testGrid.CellForPoint(image.Point{X: -1, Y: -1}); got != nil {
		t.Errorf("CellForPoint outside the grid = %q, want nil", got.Coordinate())
	}
}
