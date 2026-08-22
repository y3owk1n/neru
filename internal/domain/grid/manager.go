package grid

import (
	"image"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/domain"
)

// Manager handles variable-length grid coordinate input and manages grid state.
type Manager struct {
	domain.BaseManager

	grid          *Grid
	mainGridInput string            // This variable is just to restore the captured keys to subgrid when needed
	labelLength   int               // Length of labels (2, 3, or 4)
	onUpdate      func(redraw bool) // redraw is only used for exiting subgrid
	onShowSub     func(cell *Cell)
	inSubgrid     bool
	selectedCell  *Cell
	// Subgrid configuration
	subDims domain.GridDimensions
	subKeys string
}

// NewManager creates a new grid manager with the specified configuration.
//
// subDims is the shape of the subgrid a cell opens into. It is one value rather
// than a row count beside a column count because it is handed straight to
// SubgridCells, which decides where a subgrid key sends the cursor: an adjacent
// pair here would just be the transposition SubgridCells no longer has, one
// call up (#1294).
func NewManager(
	grid *Grid,
	subDims domain.GridDimensions,
	subKeys string,
	onUpdate func(redraw bool),
	onShowSub func(cell *Cell),
	logger *zap.Logger,
) *Manager {
	// Determine label length from first cell (if grid exists)
	labelLength := 3 // Default
	if grid != nil && len(grid.Cells()) > 0 {
		labelLength = len(grid.Cells()[0].Coordinate())
	}

	return &Manager{
		BaseManager: domain.BaseManager{
			Logger: logger,
		},
		grid:        grid,
		labelLength: labelLength,
		onUpdate:    onUpdate,
		onShowSub:   onShowSub,
		subDims:     subDims,
		subKeys:     string(SubgridKeys(subKeys, subDims.CellCount())),
	}
}

// HandleInput processes variable-length coordinate input and returns the target point when complete.
// Handles reset key, backspace, subgrid selection, input validation, and main grid navigation.
// Completion occurs when labelLength characters are entered or a subgrid selection is made.
// Returns (point, true) when selection is complete, (zero point, false) otherwise.
func (m *Manager) HandleInput(key string) (image.Point, bool) {
	// Ignore keys that are not single characters or not in the configured characters, except reset
	upper := strings.ToUpper(key)

	allowed := false
	if m.inSubgrid {
		allowed = strings.Contains(m.subKeys, upper)
	} else if m.grid != nil {
		allowed = strings.Contains(m.grid.ValidCharacters(), upper)
	}

	if len(key) != 1 || !allowed {
		return image.Point{}, false
	}

	// Cache uppercase conversion once
	upperKey := strings.ToUpper(key)

	// For main grid, ensure the new input would match at least one coordinate.
	if !m.inSubgrid && m.grid != nil {
		newInput := m.CurrentInput() + upperKey
		if !m.hasMatchingCoordinate(newInput) {
			return image.Point{}, false
		}
	}

	// If in subgrid mode, delegate to subgrid selection handler
	if m.inSubgrid && m.selectedCell != nil {
		return m.handleSubgridSelection(upperKey)
	}

	// Note: reset key already handled above (supports modifiers and single chars).

	if !m.validateInputKey(upperKey) {
		return image.Point{}, false
	}

	m.SetCurrentInput(m.CurrentInput() + upperKey)

	// Transition to subgrid when main grid coordinate is complete
	if !m.inSubgrid && len(m.CurrentInput()) >= m.labelLength {
		return m.handleLabelLengthReached()
	}

	if m.onUpdate != nil {
		m.onUpdate(false)
	}

	return image.Point{}, false
}

// CurrentGrid returns the grid.
func (m *Manager) CurrentGrid() *Grid {
	return m.grid
}

// Reset resets the input state and triggers a redraw via the onUpdate callback.
func (m *Manager) Reset() {
	m.ResetSilent()

	if m.onUpdate != nil {
		m.onUpdate(false)
	}
}

// ResetSilent resets the input state without triggering the onUpdate callback.
func (m *Manager) ResetSilent() {
	m.SetCurrentInput("")
	m.mainGridInput = ""
	m.inSubgrid = false
	m.selectedCell = nil
}

// Grid returns the grid.
func (m *Manager) Grid() *Grid {
	return m.grid
}

// UpdateGrid updates the grid used by the manager.
func (m *Manager) UpdateGrid(g *Grid) {
	m.grid = g
	// Update label length based on new grid
	if g != nil && len(g.Cells()) > 0 {
		m.labelLength = len(g.Cells()[0].Coordinate())
	}
}

// UpdateSubKeys updates the subgrid keys used for subgrid selection. It keeps
// the set an overlay draws (SubgridKeys), so the keys accepted here are the
// keys the user can see.
func (m *Manager) UpdateSubKeys(subKeys string) {
	m.subKeys = string(SubgridKeys(subKeys, m.subDims.CellCount()))
}

// HandleBackspace applies grid backspace behavior: delete one input character,
// or exit subgrid and restore main-grid input context when appropriate.
func (m *Manager) HandleBackspace() (image.Point, bool) {
	if len(m.CurrentInput()) > 0 {
		m.SetCurrentInput(m.CurrentInput()[:len(m.CurrentInput())-1])

		if m.onUpdate != nil {
			m.onUpdate(false)
		}

		return image.Point{}, false
	}

	// If in subgrid, backspace exits subgrid and back to main grid
	if m.inSubgrid {
		m.inSubgrid = false
		m.selectedCell = nil
		// Restore main grid input
		if len(m.mainGridInput) > 0 {
			// remove the last character
			m.SetCurrentInput(m.mainGridInput[:len(m.mainGridInput)-1])
		} else {
			// just in case
			m.SetCurrentInput("")
		}

		if m.onUpdate != nil {
			m.onUpdate(true)
		}
	}

	return image.Point{}, false
}

// MoveDirection slides the selected cell count cells in dir and redraws the
// subgrid over the new cell. It only applies while a subgrid is open — before
// that no cell is selected, so there is nothing to move.
//
// Returns the resulting cell center and whether anything moved.
func (m *Manager) MoveDirection(dir domain.Direction, count int) (image.Point, bool) {
	if !m.inSubgrid || m.selectedCell == nil || m.grid == nil {
		return image.Point{}, false
	}

	count = max(count, 1)

	moved := false

	for range count {
		next := m.neighbourCell(m.selectedCell, dir)
		if next == nil {
			break
		}

		m.selectedCell = next
		// Backspace out of a subgrid restores the main-grid input from
		// mainGridInput, so it has to track the cell we actually landed on.
		m.mainGridInput = next.Coordinate()
		moved = true
	}

	center := m.selectedCell.Center()

	m.Logger.Debug("Grid directional move",
		zap.String("direction", dir.String()),
		zap.Int("count", count),
		zap.Bool("moved", moved))

	if !moved {
		return center, false
	}

	// A subgrid selection is a single keypress, so no partial input survives
	// the move.
	m.SetCurrentInput("")

	if m.onShowSub != nil {
		m.onShowSub(m.selectedCell)
	}

	return center, true
}

// neighbourCell returns the cell one cell-width from cell in dir, or nil when
// that lands outside the grid.
func (m *Manager) neighbourCell(cell *Cell, dir domain.Direction) *Cell {
	bounds := cell.Bounds()
	if bounds.Empty() {
		return nil
	}

	deltaX, deltaY := dir.Delta()
	next := bounds.Add(image.Point{X: deltaX * bounds.Dx(), Y: deltaY * bounds.Dy()})

	// Probe the middle of the shifted rectangle. Truncating division rather
	// than the cell's stored center, which rounds up and would sit outside a
	// one-pixel-wide or one-pixel-tall rectangle and resolve to the wrong cell.
	target := image.Point{
		X: next.Min.X + next.Dx()/CenterDivisor,
		Y: next.Min.Y + next.Dy()/CenterDivisor,
	}

	if !target.In(m.grid.Bounds()) {
		return nil
	}

	return m.grid.CellForPoint(target)
}

// hasMatchingCoordinate checks if any grid cell coordinate starts with the given prefix.
func (m *Manager) hasMatchingCoordinate(prefix string) bool {
	if m.grid == nil {
		return false
	}

	return m.grid.HasCoordinatePrefix(prefix)
}

// handleLabelLengthReached handles the case when label length is reached.
func (m *Manager) handleLabelLengthReached() (image.Point, bool) {
	coordinate := m.CurrentInput()[:m.labelLength]
	if m.grid != nil {
		cell := m.grid.CellByCoordinate(coordinate)
		if cell != nil {
			if !m.inSubgrid {
				center := cell.center

				m.inSubgrid = true
				m.selectedCell = cell
				// Save the main grid input for restoring after subgrid
				m.mainGridInput = m.CurrentInput()
				m.SetCurrentInput("")

				if m.onShowSub != nil {
					m.onShowSub(cell)
				}

				// Return false for completion since we're entering subgrid, not completing selection
				return image.Point{X: center.X, Y: center.Y}, false
			}
		}
	}
	// Invalid coordinate, reset
	m.Reset()

	return image.Point{}, false
}

// validateInputKey validates the input key.
func (m *Manager) validateInputKey(key string) bool {
	if m.inSubgrid {
		return strings.Contains(m.subKeys, key)
	} else if m.grid != nil {
		return strings.Contains(m.grid.ValidCharacters(), key)
	}

	return false
}

// handleSubgridSelection handles subgrid selection.
// Maps the input key to a 3x3 subgrid position, calculates the precise point within the selected cell,
// and returns the final target coordinates. Completes the selection process.
func (m *Manager) handleSubgridSelection(key string) (image.Point, bool) {
	// Find the index of the key in subgrid keys
	keyIndex := strings.Index(m.subKeys, key)
	if keyIndex < 0 {
		return image.Point{}, false
	}
	// The cells this subgrid is divided into, which are the cells every overlay
	// backend draws (internal/domain/grid/subgrid_cells.go), so the cursor
	// lands in the cell the key was written on.
	cells := SubgridCells(m.selectedCell.bounds, m.subDims)

	// A key past the last cell names nothing. The bound is this manager's own
	// cell count rather than the shipped subgrid's, because the manager is
	// built with the row and column count it was handed: a fixed bound would
	// refuse keys a larger subgrid had drawn.
	if keyIndex >= len(cells) {
		return image.Point{}, false
	}

	// Calculate center point of the selected subgrid cell, rounded to nearest pixel
	selected := cells[keyIndex]
	xCoordinate := selected.Min.X + gridRound(selected.Dx())
	yCoordinate := selected.Min.Y + gridRound(selected.Dy())
	m.Logger.Debug("Grid manager: Subgrid selection complete",
		zap.Int("row", keyIndex/m.subDims.Cols), zap.Int("col", keyIndex%m.subDims.Cols),
		zap.Int("x", xCoordinate), zap.Int("y", yCoordinate))
	// m.Reset()
	return image.Point{X: xCoordinate, Y: yCoordinate}, true
}
