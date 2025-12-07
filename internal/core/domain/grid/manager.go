package grid

import (
	"image"
	"strings"

	"github.com/y3owk1n/neru/internal/core/domain"
	"go.uber.org/zap"
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
	subRows int
	subCols int
	subKeys string
}

// NewManager initializes a new grid manager with the specified configuration.
func NewManager(
	grid *Grid,
	subRows int,
	subCols int,
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
		subRows:     subRows,
		subCols:     subCols,
		subKeys:     strings.ToUpper(strings.TrimSpace(subKeys)),
	}
}

// HandleInput processes variable-length coordinate input and returns the target point when complete.
// Handles reset key, backspace, subgrid selection, input validation, and main grid navigation.
// Completion occurs when labelLength characters are entered or a subgrid selection is made.
// Returns (point, true) when selection is complete, (zero point, false) otherwise.
func (m *Manager) HandleInput(key string) (image.Point, bool) {
	resetKey := "<"

	// Handle reset key to clear input and return to initial state
	if key == resetKey {
		m.handleResetKey(false)
	}

	// Handle backspace for input correction
	if key == "\x7f" || key == "delete" || key == "backspace" {
		return m.handleBackspace()
	}

	// Ignore keys that are not single characters or not in the configured characters, except reset
	upper := strings.ToUpper(key)

	allowed := false
	if m.inSubgrid {
		allowed = strings.Contains(m.subKeys, upper)
	} else if m.grid != nil {
		allowed = strings.Contains(m.grid.ValidCharacters(), upper)
	}

	if len(key) != 1 || (key != resetKey && !allowed) {
		return image.Point{}, false
	}

	// Cache uppercase conversion once
	upperKey := strings.ToUpper(key)

	// If in subgrid mode, delegate to subgrid selection handler
	if m.inSubgrid && m.selectedCell != nil {
		return m.handleSubgridSelection(upperKey)
	}

	// Allow < to reset grid with redraw
	if upperKey == resetKey {
		m.handleResetKey(true)

		return image.Point{}, false
	}

	// Validate input key against grid characters
	if !m.validateInputKey(upperKey) {
		return image.Point{}, false
	}

	m.SetCurrentInput(m.CurrentInput() + upperKey)

	// Transition to subgrid when main grid coordinate is complete
	if !m.inSubgrid && len(m.CurrentInput()) >= m.labelLength {
		return m.handleLabelLengthReached()
	}

	// Update overlay to highlight matched cells
	if m.onUpdate != nil {
		m.onUpdate(false)
	}

	return image.Point{}, false
}

// CurrentGrid returns the grid.
func (m *Manager) CurrentGrid() *Grid {
	return m.grid
}

// Reset resets the input state.
func (m *Manager) Reset() {
	m.SetCurrentInput("")
	m.mainGridInput = ""
	m.inSubgrid = false
	m.selectedCell = nil

	if m.onUpdate != nil {
		m.onUpdate(false)
	}
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

// UpdateSubKeys updates the subgrid keys used for subgrid selection.
func (m *Manager) UpdateSubKeys(subKeys string) {
	m.subKeys = strings.ToUpper(strings.TrimSpace(subKeys))
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
	// Validate key index for 3x3 subgrid
	if keyIndex >= MaxKeyIndex {
		return image.Point{}, false
	}

	// Convert linear index to 2D subgrid coordinates
	rowIndex := keyIndex / m.subCols
	colIndex := keyIndex % m.subCols
	cellBounds := m.selectedCell.bounds

	// Compute subgrid breakpoints for even division of the cell
	xBreaks := make([]int, m.subCols+1)
	yBreaks := make([]int, m.subRows+1)
	xBreaks[0] = cellBounds.Min.X
	yBreaks[0] = cellBounds.Min.Y

	for breakIndex := 1; breakIndex <= m.subCols; breakIndex++ {
		val := float64(breakIndex) * float64(cellBounds.Dx()) / float64(m.subCols)
		xBreaks[breakIndex] = cellBounds.Min.X + int(val+RoundingFactor)
	}

	for breakIndex := 1; breakIndex <= m.subRows; breakIndex++ {
		val := float64(breakIndex) * float64(cellBounds.Dy()) / float64(m.subRows)
		yBreaks[breakIndex] = cellBounds.Min.Y + int(val+RoundingFactor)
	}

	// Ensure exact coverage of cell bounds
	xBreaks[m.subCols] = cellBounds.Max.X
	yBreaks[m.subRows] = cellBounds.Max.Y

	// Calculate center point of the selected subgrid cell
	left := xBreaks[colIndex]
	right := xBreaks[colIndex+1]
	top := yBreaks[rowIndex]
	bottom := yBreaks[rowIndex+1]
	xCoordinate := left + (right-left)/CenterDivisor
	yCoordinate := top + (bottom-top)/CenterDivisor
	m.Logger.Info("Grid manager: Subgrid selection complete",
		zap.Int("row", rowIndex), zap.Int("col", colIndex),
		zap.Int("x", xCoordinate), zap.Int("y", yCoordinate))
	// m.Reset()
	return image.Point{X: xCoordinate, Y: yCoordinate}, true
}

func (m *Manager) handleBackspace() (image.Point, bool) {
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

func (m *Manager) handleResetKey(redraw bool) {
	m.Reset()

	if m.onUpdate != nil {
		m.onUpdate(redraw)
	}
}
