package grid

import (
	"image"
	"strings"

	"go.uber.org/zap"
)

// Manager handles variable-length grid coordinate input and manages grid state.
type Manager struct {
	grid          *Grid
	currentInput  string
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
	logger  *zap.Logger
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
	if grid != nil && len(grid.GetCells()) > 0 {
		labelLength = len(grid.GetCells()[0].GetCoordinate())
	}

	return &Manager{
		grid:        grid,
		labelLength: labelLength,
		onUpdate:    onUpdate,
		onShowSub:   onShowSub,
		subRows:     subRows,
		subCols:     subCols,
		subKeys:     strings.ToUpper(strings.TrimSpace(subKeys)),
		logger:      logger,
	}
}

// HandleInput processes variable-length coordinate input and returns the target point when complete.
// Completion occurs when labelLength characters are entered or a subgrid selection is made.
func (m *Manager) HandleInput(key string) (image.Point, bool) {
	resetKey := "<"

	if key == resetKey {
		m.handleResetKey(false)
	}

	// Handle backspace
	if key == "\x7f" || key == "delete" || key == "backspace" {
		return m.handleBackspace()
	}

	// Ignore non-letter keys
	if len(key) != 1 || !isLetter(key[0]) && key != resetKey {
		return image.Point{}, false
	}

	// Cache uppercase conversion once
	upperKey := strings.ToUpper(key)

	// If we're in subgrid selection, next key chooses a subcell
	if m.inSubgrid && m.selectedCell != nil {
		return m.handleSubgridSelection(upperKey)
	}

	// Allow < to reset grid
	if upperKey == resetKey {
		m.handleResetKey(true)

		return image.Point{}, false
	}

	// Validate the input key
	if !m.validateInputKey(upperKey) {
		return image.Point{}, false
	}

	m.currentInput += upperKey

	// After reaching label length, show subgrid inside the selected cell
	if !m.inSubgrid && len(m.currentInput) >= m.labelLength {
		return m.handleLabelLengthReached()
	}

	// Update overlay to show matched cells
	if m.onUpdate != nil {
		m.onUpdate(false)
	}

	return image.Point{}, false
}

// GetInput returns the current partial coordinate input.
func (m *Manager) GetInput() string {
	return m.currentInput
}

// GetCurrentGrid returns the grid.
func (m *Manager) GetCurrentGrid() *Grid {
	return m.grid
}

// Reset resets the input state.
func (m *Manager) Reset() {
	m.currentInput = ""
	m.mainGridInput = ""
	m.inSubgrid = false
	m.selectedCell = nil

	if m.onUpdate != nil {
		m.onUpdate(false)
	}
}

// GetGrid returns the grid.
func (m *Manager) GetGrid() *Grid {
	return m.grid
}

// UpdateGrid updates the grid used by the manager.
func (m *Manager) UpdateGrid(g *Grid) {
	m.grid = g
	// Update label length based on new grid
	if g != nil && len(g.GetCells()) > 0 {
		m.labelLength = len(g.GetCells()[0].GetCoordinate())
	}
}

// UpdateSubKeys updates the subgrid keys used for subgrid selection.
func (m *Manager) UpdateSubKeys(subKeys string) {
	m.subKeys = strings.ToUpper(strings.TrimSpace(subKeys))
}

// handleLabelLengthReached handles the case when label length is reached.
func (m *Manager) handleLabelLengthReached() (image.Point, bool) {
	coordinate := m.currentInput[:m.labelLength]
	if m.grid != nil {
		cell := m.grid.GetCellByCoordinate(coordinate)
		if cell != nil {
			if !m.inSubgrid {
				center := cell.Center

				m.inSubgrid = true
				m.selectedCell = cell
				// Save the main grid input for restoring after subgrid
				m.mainGridInput = m.currentInput
				m.currentInput = ""

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
	// Check if character is valid for grid
	if m.grid != nil && !strings.Contains(m.grid.GetCharacters(), key) {
		return false
	}

	// Check if this key could potentially lead to a valid coordinate
	// by checking if there's any cell that starts with currentInput + key
	potentialInput := m.currentInput + key
	validPrefix := false

	for _, cell := range m.grid.GetCells() {
		if len(cell.GetCoordinate()) >= len(potentialInput) &&
			strings.HasPrefix(cell.GetCoordinate(), potentialInput) {
			validPrefix = true

			break
		}
	}

	// If this key doesn't lead to any valid coordinate, ignore it
	if !validPrefix {
		return false
	}

	return true
}

// handleSubgridSelection handles subgrid selection.
func (m *Manager) handleSubgridSelection(key string) (image.Point, bool) {
	keyIndex := strings.Index(m.subKeys, key)
	if keyIndex < 0 {
		return image.Point{}, false
	}
	// Subgrid is always 3x3
	if keyIndex >= 9 {
		return image.Point{}, false
	}

	rowIndex := keyIndex / m.subCols
	colIndex := keyIndex % m.subCols
	cellBounds := m.selectedCell.Bounds
	// Compute breakpoints to match overlay splitting and cover full bounds
	xBreaks := make([]int, m.subCols+1)
	yBreaks := make([]int, m.subRows+1)
	xBreaks[0] = cellBounds.Min.X

	yBreaks[0] = cellBounds.Min.Y
	for breakIndex := 1; breakIndex <= m.subCols; breakIndex++ {
		val := float64(breakIndex) * float64(cellBounds.Dx()) / float64(m.subCols)
		xBreaks[breakIndex] = cellBounds.Min.X + int(val+0.5)
	}

	for breakIndex := 1; breakIndex <= m.subRows; breakIndex++ {
		val := float64(breakIndex) * float64(cellBounds.Dy()) / float64(m.subRows)
		yBreaks[breakIndex] = cellBounds.Min.Y + int(val+0.5)
	}
	// Ensure exact coverage
	xBreaks[m.subCols] = cellBounds.Max.X
	yBreaks[m.subRows] = cellBounds.Max.Y
	left := xBreaks[colIndex]
	right := xBreaks[colIndex+1]
	top := yBreaks[rowIndex]
	bottom := yBreaks[rowIndex+1]
	xCoordinate := left + (right-left)/2
	yCoordinate := top + (bottom-top)/2
	m.logger.Info("Grid manager: Subgrid selection complete",
		zap.Int("row", rowIndex), zap.Int("col", colIndex),
		zap.Int("x", xCoordinate), zap.Int("y", yCoordinate))
	// m.Reset()
	return image.Point{X: xCoordinate, Y: yCoordinate}, true
}

func (m *Manager) handleBackspace() (image.Point, bool) {
	if len(m.currentInput) > 0 {
		m.currentInput = m.currentInput[:len(m.currentInput)-1]

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
			m.currentInput = m.mainGridInput[:len(m.mainGridInput)-1]
		} else {
			// just in case
			m.currentInput = ""
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

func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
