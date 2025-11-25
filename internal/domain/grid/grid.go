package grid

import (
	"image"
	"math"
	"strings"

	"go.uber.org/zap"
)

// Grid represents a coordinate grid system for spatial navigation with optimized cell sizing.
type Grid struct {
	characters string          // Characters used for coordinates (e.g., "asdfghjkl")
	bounds     image.Rectangle // Screen bounds
	cells      []*Cell         // All cells with 3-char coordinates
	index      map[string]*Cell
}

// Cell represents a grid cell containing coordinate, bounds, and center point information.
type Cell struct {
	coordinate string          // 3-character coordinate (e.g., "AAA", "ABC")
	bounds     image.Rectangle // Cell bounds
	center     image.Point     // Center point
}

// GetCoordinate returns the 3-character coordinate.
func (c *Cell) GetCoordinate() string { return c.coordinate }

// GetBounds returns the cell bounds.
func (c *Cell) GetBounds() image.Rectangle { return c.bounds }

// GetCenter returns the center point.
func (c *Cell) GetCenter() image.Point { return c.center }

// NewGrid creates a grid with automatically optimized cell sizes for the screen.
// Cell sizes are dynamically calculated based on screen dimensions, resolution, and aspect ratio
// to ensure optimal precision and usability across all display types.
//
// Grid layout uses spatial regions for predictable navigation:
//   - Each region is identified by the first character (Region A, Region B, etc.)
//   - Within each region, coordinates flow left-to-right, top-to-bottom
//   - Region A: AAA, ABA, ACA (left-to-right), then AAB, ABB, ACB (next row)
//   - Regions flow left-to-right until screen width is filled
//   - Next region starts on new row below, continuing the pattern
//   - This allows users to think: "C** coordinates are in region C on the screen"
//
// Cell sizing is fully automatic based on screen characteristics:
//   - Very small screens (<1.5M pixels): 25-60px cells for maximum precision
//   - Small-medium screens (1.5-2.5M pixels): 30-80px cells
//   - Medium-large screens (2.5-4M pixels): 40-100px cells
//   - Very large screens (>4M pixels): 50-120px cells
func NewGrid(characters string, bounds image.Rectangle, logger *zap.Logger) *Grid {
	logger.Debug("Creating new grid",
		zap.String("characters", characters),
		zap.Int("bounds_width", bounds.Dx()),
		zap.Int("bounds_height", bounds.Dy()))

	if characters == "" {
		characters = "abcdefghijklmnopqrstuvwxyz"
	}
	// Cache uppercase conversion once at the start
	uppercaseChars := strings.ToUpper(characters)
	chars := []rune(uppercaseChars)
	numChars := len(chars)

	// Ensure we have valid characters
	if numChars < 2 {
		uppercaseChars = strings.ToUpper("abcdefghijklmnopqrstuvwxyz")
		chars = []rune(uppercaseChars)
		numChars = len(chars)
	}

	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	logger.Debug("Grid dimensions calculated",
		zap.Int("width", width),
		zap.Int("height", height))

	if gridCacheEnabled {
		if cells, ok := gridCache.get(uppercaseChars, bounds); ok {
			logger.Debug("Grid cache hit",
				zap.Int("cell_count", len(cells)))

			return &Grid{characters: uppercaseChars, bounds: bounds, cells: cells}
		}

		logger.Debug("Grid cache miss")
	}

	if width <= 0 || height <= 0 {
		logger.Warn("Invalid grid bounds, creating minimal grid",
			zap.Int("width", width),
			zap.Int("height", height))

		return &Grid{
			characters: uppercaseChars,
			bounds:     bounds,
			cells:      []*Cell{},
		}
	}

	// Automatically determine optimal cell size constraints based on screen characteristics
	minCellSize, maxCellSize := calculateOptimalCellSizes(width, height)

	// Find all valid grid configurations and pick the one with best aspect ratio match
	candidates := findValidGridConfigurations(width, height, minCellSize, maxCellSize)

	// Pick the candidate with the best (lowest) score
	gridCols, gridRows := selectBestCandidate(candidates, width, height, minCellSize, maxCellSize)

	// Safety check: ensure we always have at least a 2x2 grid
	if gridCols < 2 {
		gridCols = 2
	}

	if gridRows < 2 {
		gridRows = 2
	}

	// Calculate total cells needed to fill screen
	totalCells := gridRows * gridCols

	// Calculate maximum possible cells we can label
	maxPossibleCells := numChars * numChars * numChars * numChars

	// Cap totalCells to what we can actually label
	if totalCells > maxPossibleCells {
		totalCells = maxPossibleCells
		gridCols = gridMax(int(math.Sqrt(float64(totalCells)*float64(width)/float64(height))), 1)
		gridRows = gridMax(totalCells/gridCols, 1)
		totalCells = gridRows * gridCols
	}

	// Determine optimal label length based on total cells
	labelLength := calculateLabelLength(totalCells, numChars)

	// Calculate base cell sizes and remainders
	baseCellWidth := width / gridCols
	baseCellHeight := height / gridRows
	remainderWidth := width % gridCols
	remainderHeight := height % gridRows

	// Generate cells with spatial region logic
	cells := generateCellsWithRegions(chars, numChars, gridCols, gridRows, labelLength,
		bounds, baseCellWidth, baseCellHeight, remainderWidth, remainderHeight, logger)

	logger.Debug("Grid created successfully",
		zap.Int("cell_count", len(cells)),
		zap.Int("grid_cols", gridCols),
		zap.Int("grid_rows", gridRows),
		zap.Int("label_length", labelLength))

	if gridCacheEnabled {
		gridCache.put(uppercaseChars, bounds, cells)
		logger.Debug("Grid cache store",
			zap.Int("cell_count", len(cells)))
	}

	// Pre-allocate index map with exact capacity
	index := make(map[string]*Cell, len(cells))
	for _, cell := range cells {
		index[cell.GetCoordinate()] = cell
	}

	return &Grid{
		characters: uppercaseChars,
		bounds:     bounds,
		cells:      cells,
		index:      index,
	}
}

// GetCharacters returns the characters used for coordinates.
func (g *Grid) GetCharacters() string { return g.characters }

// GetBounds returns the screen bounds.
func (g *Grid) GetBounds() image.Rectangle { return g.bounds }

// GetCells returns all cells with 3-char coordinates.
func (g *Grid) GetCells() []*Cell { return g.cells }

// GetIndex returns the cell index map.
func (g *Grid) GetIndex() map[string]*Cell { return g.index }

// generateCellsWithRegions creates cells using spatial region logic.
// Each region (identified by first char) fills left-to-right, top-to-bottom.
// Regions flow across screen, wrapping to next row when width is exhausted.
func generateCellsWithRegions(chars []rune, numChars, gridCols, gridRows, labelLength int,
	bounds image.Rectangle, baseCellWidth, baseCellHeight, remainderWidth, remainderHeight int,
	logger *zap.Logger,
) []*Cell {
	logger.Debug("Generating cells with regions",
		zap.Int("num_chars", numChars),
		zap.Int("grid_cols", gridCols),
		zap.Int("grid_rows", gridRows),
		zap.Int("label_length", labelLength))

	cells := make([]*Cell, gridCols*gridRows)
	cellIndex := 0

	// Calculate region dimensions (how many cols/rows per region)
	// Each region is a sub-grid of size numChars x numChars
	var regionCols, regionRows int

	// Adjust region size based on label length
	switch labelLength {
	case 2:
		// For 2-char labels: each region is numChars x numChars
		regionCols = numChars
		regionRows = numChars
	case 3:
		// For 3-char labels: first char = region, next 2 chars = position
		// Region is numChars wide x numChars tall
		regionCols = numChars
		regionRows = numChars
	default:
		// For 4-char labels: first 2 chars could represent super-regions
		regionCols = numChars
		regionRows = numChars
	}

	// Track current position as we fill regions
	currentCol := 0
	currentRow := 0

	// Iterate through regions (first character)
	regionIndex := 0
	maxRegions := numChars * numChars // Maximum regions we might need

	// Precompute x/y starts to avoid inner summation loops
	xStarts := make([]int, gridCols)
	yStarts := make([]int, gridRows)

	for colIndex := range xStarts {
		xStarts[colIndex] = bounds.Min.X + colIndex*baseCellWidth
		if colIndex < remainderWidth {
			xStarts[colIndex] += colIndex
		} else {
			xStarts[colIndex] += remainderWidth
		}
	}

	for rowIndex := range yStarts {
		yStarts[rowIndex] = bounds.Min.Y + rowIndex*baseCellHeight
		if rowIndex < remainderHeight {
			yStarts[rowIndex] += rowIndex
		} else {
			yStarts[rowIndex] += remainderHeight
		}
	}

	for regionIndex < maxRegions && currentRow < gridRows {
		// Determine region identifier (first character)
		var regionChar1, regionChar2 rune

		switch labelLength {
		case 2:
			regionChar1 = chars[regionIndex%numChars]
		case 3:
			regionChar1 = chars[regionIndex%numChars]
		default: // 4 chars
			regionChar1 = chars[regionIndex/numChars%numChars]
			regionChar2 = chars[regionIndex%numChars]
		}

		// Calculate how many columns this region can occupy
		colsAvailable := gridCols - currentCol
		colsForRegion := gridMin(regionCols, colsAvailable)

		// Calculate how many rows this region can occupy
		rowsAvailable := gridRows - currentRow
		rowsForRegion := gridMin(regionRows, rowsAvailable)

		// Fill this region
		for rowIndex := range rowsForRegion {
			for colIndex := range colsForRegion {
				globalCol := currentCol + colIndex
				globalRow := currentRow + rowIndex

				if globalCol >= gridCols || globalRow >= gridRows {
					break
				}

				// Generate coordinate for this cell
				// Second char = column within region, third char = row within region
				var coordinate string

				switch labelLength {
				case 2:
					// Use strings.Builder for efficient string concatenation
					var stringBuilder strings.Builder
					stringBuilder.Grow(2)
					stringBuilder.WriteRune(regionChar1)
					stringBuilder.WriteRune(chars[colIndex])
					coordinate = stringBuilder.String()
				case 3:
					// First char = region, second char = column, third char = row
					char2 := chars[colIndex%numChars] // column
					char3 := chars[rowIndex%numChars] // row

					var stringBuilder strings.Builder
					stringBuilder.Grow(3)
					stringBuilder.WriteRune(regionChar1)
					stringBuilder.WriteRune(char2)
					stringBuilder.WriteRune(char3)
					coordinate = stringBuilder.String()
				default: // 4 chars
					// First 2 chars = region, third char = column, fourth char = row
					char3 := chars[colIndex%numChars] // column
					char4 := chars[rowIndex%numChars] // row

					var stringBuilder strings.Builder
					stringBuilder.Grow(4)
					stringBuilder.WriteRune(regionChar1)
					stringBuilder.WriteRune(regionChar2)
					stringBuilder.WriteRune(char3)
					stringBuilder.WriteRune(char4)
					coordinate = stringBuilder.String()
				}

				// Calculate cell dimensions with remainder distribution
				cellWidth := baseCellWidth
				if globalCol < remainderWidth {
					cellWidth++
				}

				cellHeight := baseCellHeight
				if globalRow < remainderHeight {
					cellHeight++
				}

				xCoordinate := xStarts[globalCol]
				yCoordinate := yStarts[globalRow]

				cell := &Cell{
					coordinate: coordinate,
					bounds: image.Rect(
						xCoordinate, yCoordinate,
						xCoordinate+cellWidth, yCoordinate+cellHeight,
					),
					center: image.Point{
						X: xCoordinate + cellWidth/2,
						Y: yCoordinate + cellHeight/2,
					},
				}
				cells[cellIndex] = cell
				cellIndex++
			}
		}

		// Move to next region position
		currentCol += colsForRegion

		// If we've filled the row width, move to next row
		if currentCol >= gridCols {
			currentCol = 0
			currentRow += rowsForRegion
		}

		regionIndex++

		// Stop if we've filled the entire screen
		if cellIndex >= gridCols*gridRows {
			break
		}
	}

	// Return only the filled portion of the slice
	return cells[:cellIndex]
}

// candidate represents a valid grid configuration.
type candidate struct {
	cols, rows   int
	cellW, cellH int
	score        float64
}

// calculateOptimalCellSizes determines optimal cell size constraints based on screen characteristics.
func calculateOptimalCellSizes(width, height int) (int, int) {
	screenArea := width * height
	screenAspect := float64(width) / float64(height)

	var minCellSize, maxCellSize int

	// Calculate optimal cell size ranges based on screen size and pixel density
	switch {
	case screenArea < 1500000:
		minCellSize = 30
		maxCellSize = 60
	case screenArea < 2500000:
		minCellSize = 30
		maxCellSize = 80
	case screenArea < 4000000:
		minCellSize = 40
		maxCellSize = 100
	default:
		minCellSize = 50
		maxCellSize = 120
	}

	// Adjust cell size constraints for extreme aspect ratios
	if screenAspect > 2.5 || screenAspect < 0.4 {
		maxCellSize = int(float64(maxCellSize) * 1.2)
	}

	return minCellSize, maxCellSize
}

// calculateLabelLength determines the optimal label length based on total cells.
func calculateLabelLength(totalCells, numChars int) int {
	switch {
	case totalCells <= numChars*numChars:
		return 2
	case totalCells <= numChars*numChars*numChars:
		return 3
	default:
		return 4
	}
}

// selectBestCandidate picks the candidate with the best (lowest) score.
func selectBestCandidate(
	candidates []candidate,
	width, height, minCellSize, maxCellSize int,
) (int, int) {
	var gridCols, gridRows int

	if len(candidates) > 0 {
		best := candidates[0]
		for _, cand := range candidates[1:] {
			if cand.score < best.score {
				best = cand
			}
		}

		gridCols = best.cols
		gridRows = best.rows
	} else {
		// Fallback: if no valid candidates, use simple best-fit approach
		findBestFit := func(dimension, minSize, maxSize int) int {
			count := gridMax(dimension/minSize, 1)
			for dimension/count > maxSize {
				count++
			}

			return count
		}
		gridCols = findBestFit(width, minCellSize, maxCellSize)
		gridRows = findBestFit(height, minCellSize, maxCellSize)
	}

	return gridCols, gridRows
}

// findValidGridConfigurations searches through all valid grid configurations.
func findValidGridConfigurations(width, height, minCellSize, maxCellSize int) []candidate {
	var candidates []candidate

	// Calculate search ranges
	minCols := max(width/maxCellSize, 1)
	maxCols := max(width/minCellSize, 1)

	minRows := max(height/maxCellSize, 1)
	maxRows := max(height/minCellSize, 1)

	// Search through all valid grid configurations
	for colIndex := maxCols; colIndex >= minCols && colIndex >= 1; colIndex-- {
		cellWidth := width / colIndex
		if cellWidth < minCellSize || cellWidth > maxCellSize {
			continue
		}

		for rowIndex := maxRows; rowIndex >= minRows && rowIndex >= 1; rowIndex-- {
			cellHeight := height / rowIndex
			if cellHeight < minCellSize || cellHeight > maxCellSize {
				continue
			}

			// Calculate cell aspect ratio
			cellAspect := float64(cellWidth) / float64(cellHeight)

			// Score based on how close cell is to being square
			aspectDiff := cellAspect - 1.0
			if aspectDiff < 0 {
				aspectDiff = -aspectDiff
			}

			// Also consider cell count - prefer more cells for better precision
			totalCells := float64(colIndex * rowIndex)
			maxCells := float64(maxCols * maxRows)
			cellScore := (maxCells - totalCells) / maxCells * 0.1

			aspectScore := aspectDiff + cellScore

			candidates = append(candidates, candidate{
				cols:  colIndex,
				rows:  rowIndex,
				cellW: cellWidth,
				cellH: cellHeight,
				score: aspectScore,
			})
		}
	}

	return candidates
}

// GetAllCells returns all grid cells.
func (g *Grid) GetAllCells() []*Cell {
	return g.cells
}

// GetCellByCoordinate returns the cell for a given coordinate. (2, 3, or 4 characters).
func (g *Grid) GetCellByCoordinate(coordinate string) *Cell {
	coordinate = strings.ToUpper(coordinate)

	if g.index != nil {
		if cell, ok := g.index[coordinate]; ok {
			return cell
		}
	}

	for _, cell := range g.cells {
		if cell.GetCoordinate() == coordinate {
			return cell
		}
	}

	return nil
}

// CalculateOptimalGrid calculates optimal character count for coverage.
func CalculateOptimalGrid(characters string) (int, int) {
	// For flat 3-char grid, we don't use rows/cols
	// Just return sensible defaults (will be ignored)
	numChars := len(characters)
	if numChars < 2 {
		numChars = 9
	}

	return numChars, numChars
}

func gridMax(a, b int) int {
	if a > b {
		return a
	}

	return b
}

func gridMin(a, b int) int {
	if a < b {
		return a
	}

	return b
}
