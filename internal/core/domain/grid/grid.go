package grid

import (
	"image"
	"math"
	"strings"
	"sync"

	"go.uber.org/zap"
)

const (
	// SmallScreenArea is the threshold for small screen area.
	SmallScreenArea = 1500000
	// MediumScreenArea is the threshold for medium screen area.
	MediumScreenArea = 2500000
	// LargeScreenArea is the threshold for large screen area.
	LargeScreenArea = 4000000

	// ExtremeAspectRatioHigh is the high threshold for extreme aspect ratios.
	ExtremeAspectRatioHigh = 2.5
	// ExtremeAspectRatioLow is the low threshold for extreme aspect ratios.
	ExtremeAspectRatioLow = 0.4
	// AspectRatioAdjustment is the adjustment factor for extreme aspect ratios.
	AspectRatioAdjustment = 1.2

	// MinCharactersLength is the minimum length for characters.
	MinCharactersLength = 2

	// MinGridCols is the minimum number of grid columns.
	MinGridCols = 2

	// MinGridRows is the minimum number of grid rows.
	MinGridRows = 2

	// MaxKeyIndex is the maximum key index.
	MaxKeyIndex = 9

	// RoundingFactor is the factor for rounding.
	RoundingFactor = 0.5

	// CenterDivisor is the divisor for center calculation.
	CenterDivisor = 2

	// ScoreWeight is the weight for scoring.
	ScoreWeight = 0.1

	// StringBuilderGrow2 is the growth for string builder.
	StringBuilderGrow2 = 2

	// StringBuilderGrow3 is the growth for string builder.
	StringBuilderGrow3 = 3

	// StringBuilderGrow4 is the growth for string builder.
	StringBuilderGrow4 = 4

	// LabelLength2 is the label length 2.
	LabelLength2 = 2

	// LabelLength3 is the label length 3.
	LabelLength3 = 3

	// LabelLength4 is the label length 4.
	LabelLength4 = 4

	// CountsCapacity is the capacity for counts.
	CountsCapacity = 5

	// PrefixLengthCheck is the check for prefix length.
	PrefixLengthCheck = 2
)

// Grid represents a coordinate grid system for spatial navigation with optimized cell sizing.
type Grid struct {
	characters string          // Characters used for coordinates (e.g., "asdfghjkl")
	rowChars   []rune          // Characters used for row labels
	colChars   []rune          // Characters used for column labels
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

// Coordinate returns the 3-character coordinate.
func (c *Cell) Coordinate() string {
	return c.coordinate
}

// Bounds returns the cell bounds.
func (c *Cell) Bounds() image.Rectangle {
	return c.bounds
}

// Center returns the center point.
func (c *Cell) Center() image.Point {
	return c.center
}

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
//
// If rowLabels or colLabels are empty, they will be inferred from characters.
func NewGrid(characters string, bounds image.Rectangle, logger *zap.Logger) *Grid {
	return NewGridWithLabels(characters, "", "", bounds, logger)
}

// NewGridWithLabels creates a grid with custom row and column labels.
// If rowLabels or colLabels are empty, they will be inferred from characters.
func NewGridWithLabels(
	characters, rowLabels, colLabels string,
	bounds image.Rectangle,
	logger *zap.Logger,
) *Grid {
	logger.Debug("Creating new grid",
		zap.String("characters", characters),
		zap.String("rowLabels", rowLabels),
		zap.String("colLabels", colLabels),
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
	if numChars < MinCharactersLength {
		uppercaseChars = strings.ToUpper("abcdefghijklmnopqrstuvwxyz")
		chars = []rune(uppercaseChars)
		numChars = len(chars)
	}

	// Prepare row and column labels
	rowChars := chars
	colChars := chars

	if rowLabels != "" {
		rowChars = []rune(strings.ToUpper(rowLabels))
	}

	if colLabels != "" {
		colChars = []rune(strings.ToUpper(colLabels))
	}

	numRowChars := len(rowChars)
	numColChars := len(colChars)

	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	logger.Debug("Grid dimensions calculated",
		zap.Int("width", width),
		zap.Int("height", height))

	if gridCacheEnabled {
		if cells, ok := gridCache.get(uppercaseChars, strings.ToUpper(rowLabels), strings.ToUpper(colLabels), bounds); ok {
			logger.Debug("Grid cache hit",
				zap.Int("cell_count", len(cells)))

			// Pre-allocate index map with exact capacity
			index := make(map[string]*Cell, len(cells))
			for _, cell := range cells {
				index[cell.Coordinate()] = cell
			}

			return &Grid{
				characters: uppercaseChars,
				rowChars:   rowChars,
				colChars:   colChars,
				bounds:     bounds,
				cells:      cells,
				index:      index,
			}
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
	// This ensures cells are as square as possible for intuitive navigation
	candidates := findValidGridConfigurations(width, height, minCellSize, maxCellSize)

	// Pick the candidate with the best (lowest) score
	gridCols, gridRows := selectBestCandidate(candidates, width, height, minCellSize, maxCellSize)

	// Safety check: ensure we always have at least a 2x2 grid
	if gridCols < MinGridCols {
		gridCols = 2
	}

	if gridRows < MinGridRows {
		gridRows = 2
	}

	// Calculate total cells needed to fill screen
	totalCells := gridRows * gridCols

	// Determine optimal label length based on total cells and available characters
	labelLength := calculateLabelLength(totalCells, numChars, numRowChars, numColChars)

	// Calculate maximum possible cells we can label based on label length
	var maxPossibleCells int
	switch labelLength {
	case LabelLength2:
		maxPossibleCells = numChars * numColChars
	case LabelLength3:
		maxPossibleCells = numChars * numColChars * numRowChars
	default:
		maxPossibleCells = numChars * numChars * numColChars * numRowChars
	}

	// Cap totalCells to what we can actually label
	if totalCells > maxPossibleCells {
		// Calculate grid dimensions that fit within maxPossibleCells
		gridCols = gridMax(
			int(math.Sqrt(float64(maxPossibleCells)*float64(width)/float64(height))),
			1,
		)
		gridRows = gridMax(maxPossibleCells/gridCols, 1)
		// Update totalCells to match the actual grid dimensions
		totalCells = gridRows * gridCols //nolint:ineffassign,staticcheck,wastedassign // totalCells is used later in calculateLabelLength
	}

	// Calculate base cell sizes and remainders
	baseCellWidth := width / gridCols
	baseCellHeight := height / gridRows
	remainderWidth := width % gridCols
	remainderHeight := height % gridRows

	// Generate cells with spatial region logic
	cells := generateCellsWithRegions(
		chars,
		rowChars,
		colChars,
		numChars,
		gridCols,
		gridRows,
		labelLength,
		bounds,
		baseCellWidth,
		baseCellHeight,
		remainderWidth,
		remainderHeight,
		logger,
	)

	logger.Debug("Grid created successfully",
		zap.Int("cell_count", len(cells)),
		zap.Int("grid_cols", gridCols),
		zap.Int("grid_rows", gridRows),
		zap.Int("label_length", labelLength))

	if gridCacheEnabled {
		gridCache.put(
			uppercaseChars,
			strings.ToUpper(rowLabels),
			strings.ToUpper(colLabels),
			bounds,
			cells,
		)
		logger.Debug("Grid cache store",
			zap.Int("cell_count", len(cells)))
	}

	// Pre-allocate index map with exact capacity
	index := make(map[string]*Cell, len(cells))
	for _, cell := range cells {
		index[cell.Coordinate()] = cell
	}

	return &Grid{
		characters: uppercaseChars,
		rowChars:   rowChars,
		colChars:   colChars,
		bounds:     bounds,
		cells:      cells,
		index:      index,
	}
}

// Characters returns the characters used for coordinates.
func (g *Grid) Characters() string {
	return g.characters
}

// ValidCharacters returns all characters that can appear in grid coordinates.
func (g *Grid) ValidCharacters() string {
	charSet := make(map[rune]bool)
	for _, r := range g.characters {
		charSet[r] = true
	}

	for _, r := range g.rowChars {
		charSet[r] = true
	}

	for _, r := range g.colChars {
		charSet[r] = true
	}

	var result strings.Builder
	for r := range charSet {
		result.WriteRune(r)
	}

	return result.String()
}

// Bounds returns the screen bounds.
func (g *Grid) Bounds() image.Rectangle {
	return g.bounds
}

// Cells returns all cells with 3-char coordinates.
func (g *Grid) Cells() []*Cell {
	return g.cells
}

// Index returns the cell index map.
func (g *Grid) Index() map[string]*Cell {
	return g.index
}

// generateCellsWithRegions creates cells using spatial region logic.
// Each region (identified by first char) fills left-to-right, top-to-bottom.
// Handles variable label lengths (2, 3, or 4 chars) and distributes remainder pixels
// to ensure cells cover the entire screen bounds without gaps.
func generateCellsWithRegions(
	chars, rowChars, colChars []rune,
	numChars, gridCols, gridRows, labelLength int,
	bounds image.Rectangle,
	baseCellWidth, baseCellHeight, remainderWidth, remainderHeight int,
	logger *zap.Logger,
) []*Cell {
	logger.Debug("Generating cells with regions",
		zap.Int("num_chars", numChars),
		zap.Int("grid_cols", gridCols),
		zap.Int("grid_rows", gridRows),
		zap.Int("label_length", labelLength))

	cells := make([]*Cell, gridCols*gridRows)
	cellIndex := 0

	// Calculate region dimensions based on label length
	// Each region represents a group of cells sharing the same prefix character(s)
	var regionCols, regionRows int

	// Adjust region size based on label length and available characters
	switch labelLength {
	case LabelLength2:
		// For 2-char labels: each region is len(colChars) x 1
		regionCols = len(colChars)
		regionRows = 1
	case LabelLength3:
		// For 3-char labels: region + col + row
		regionCols = len(colChars)
		regionRows = len(rowChars)
	default:
		// For 4-char labels: region1 + region2 + col + row
		regionCols = len(colChars)
		regionRows = len(rowChars)
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

	// Iterate through regions, filling the grid left-to-right, top-to-bottom
	for regionIndex < maxRegions && currentRow < gridRows {
		// Determine region identifier character(s) based on label length
		var regionChar1, regionChar2 rune

		switch labelLength {
		case LabelLength2:
			regionChar1 = chars[regionIndex%numChars]
		case LabelLength3:
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
				case LabelLength2:
					// Use strings.Builder for efficient string concatenation
					var stringBuilder strings.Builder
					stringBuilder.Grow(StringBuilderGrow2)
					stringBuilder.WriteRune(regionChar1)
					stringBuilder.WriteRune(colChars[colIndex%len(colChars)])
					coordinate = stringBuilder.String()
				case LabelLength3:
					// First char = region, second char = column, third char = row
					char2 := colChars[colIndex%len(colChars)] // column
					char3 := rowChars[rowIndex%len(rowChars)] // row

					var stringBuilder strings.Builder
					stringBuilder.Grow(StringBuilderGrow3)
					stringBuilder.WriteRune(regionChar1)
					stringBuilder.WriteRune(char2)
					stringBuilder.WriteRune(char3)
					coordinate = stringBuilder.String()
				default: // 4 chars
					// First 2 chars = region, third char = column, fourth char = row
					char3 := colChars[colIndex%len(colChars)] // column
					char4 := rowChars[rowIndex%len(rowChars)] // row

					var stringBuilder strings.Builder
					stringBuilder.Grow(StringBuilderGrow4)
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

// Candidate represents a valid grid configuration.
type Candidate struct {
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
	case screenArea < SmallScreenArea:
		minCellSize = 30
		maxCellSize = 60
	case screenArea < MediumScreenArea:
		minCellSize = 30
		maxCellSize = 80
	case screenArea < LargeScreenArea:
		minCellSize = 40
		maxCellSize = 100
	default:
		minCellSize = 50
		maxCellSize = 120
	}

	// Adjust cell size constraints for extreme aspect ratios
	if screenAspect > ExtremeAspectRatioHigh || screenAspect < ExtremeAspectRatioLow {
		maxCellSize = int(float64(maxCellSize) * AspectRatioAdjustment)
	}

	return minCellSize, maxCellSize
}

// calculateLabelLength determines the optimal label length based on total cells and available characters.
func calculateLabelLength(totalCells, numChars, numRowChars, numColChars int) int {
	// If custom row/col labels are provided (numRowChars/numColChars != numChars), use more labels
	if numRowChars != numChars || numColChars != numChars {
		max2Char := numChars * numColChars

		max3Char := numChars * numColChars * numRowChars
		switch {
		case totalCells <= max2Char:
			return LabelLength2
		case totalCells <= max3Char:
			return LabelLength3
		default:
			return LabelLength4
		}
	}
	// Default logic when using characters for everything
	switch {
	case totalCells <= numChars*numChars:
		return LabelLength2
	case totalCells <= numChars*numChars*numChars:
		return LabelLength3
	default:
		return LabelLength4
	}
}

// selectBestCandidate picks the candidate with the best (lowest) score.
func selectBestCandidate(
	candidates []Candidate,
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
// Evaluates combinations of columns and rows within cell size constraints,
// calculating aspect ratio scores to find grids that produce square-like cells.
// Returns candidates sorted by score (lower is better).
func findValidGridConfigurations(width, height, minCellSize, maxCellSize int) []Candidate {
	var (
		candidates []Candidate
		mutex      sync.Mutex
	)

	// Calculate search ranges
	minCols := max(width/maxCellSize, 1)
	maxCols := max(width/minCellSize, 1)

	minRows := max(height/maxCellSize, 1)
	maxRows := max(height/minCellSize, 1)

	// Use WaitGroup for parallel computation
	var waitGroup sync.WaitGroup

	// Search through all valid grid configurations within constraints
	for colIndex := maxCols; colIndex >= minCols && colIndex >= 1; colIndex-- {
		waitGroup.Add(1)

		go func(col int) {
			defer waitGroup.Done()

			var localCandidates []Candidate

			cellWidth := width / col
			if cellWidth < minCellSize || cellWidth > maxCellSize {
				return
			}

			for rowIndex := maxRows; rowIndex >= minRows && rowIndex >= 1; rowIndex-- {
				cellHeight := height / rowIndex
				if cellHeight < minCellSize || cellHeight > maxCellSize {
					continue
				}

				// Calculate how square the cells are (aspect ratio deviation from 1.0)
				cellAspect := float64(cellWidth) / float64(cellHeight)

				aspectDiff := cellAspect - 1.0
				if aspectDiff < 0 {
					aspectDiff = -aspectDiff
				}

				// Prefer configurations with more cells for finer precision
				totalCells := float64(col * rowIndex)
				maxCells := float64(maxCols * maxRows)
				cellScore := (maxCells - totalCells) / maxCells * ScoreWeight

				aspectScore := aspectDiff + cellScore

				cand := Candidate{
					cols:  col,
					rows:  rowIndex,
					cellW: cellWidth,
					cellH: cellHeight,
					score: aspectScore,
				}

				localCandidates = append(localCandidates, cand)
			}

			mutex.Lock()

			candidates = append(candidates, localCandidates...)

			mutex.Unlock()
		}(colIndex)
	}

	waitGroup.Wait()

	return candidates
}

// AllCells returns all grid cells.
func (g *Grid) AllCells() []*Cell {
	return g.cells
}

// CellByCoordinate returns the cell for a given coordinate. (2, 3, or 4 characters).
func (g *Grid) CellByCoordinate(coordinate string) *Cell {
	coordinate = strings.ToUpper(coordinate)

	if g.index != nil {
		if cell, ok := g.index[coordinate]; ok {
			return cell
		}
	}

	for _, cell := range g.cells {
		if cell.Coordinate() == coordinate {
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
	if numChars < MinCharactersLength {
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
