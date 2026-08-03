package grid

import (
	"image"
	"strings"

	"go.uber.org/zap"
)

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

	// Clamp both dimensions into [1, Max]. This function takes its dimensions as
	// parameters rather than deriving them, so it cannot assume a caller has
	// bounded them: a zero row count or a negative one would otherwise reach
	// the arithmetic and the allocation below.
	//
	// The clamp is also what keeps the cell count safe. Both bounds are
	// constants, so the product is at most MaxGridCols*MaxGridRows.
	//
	// Explicit comparisons rather than min/max: this is the form the rest of
	// the file uses, and the form static analysis follows when proving the
	// bound on the allocation.
	if gridCols < 1 {
		gridCols = 1
	}

	if gridCols > MaxGridCols {
		gridCols = MaxGridCols
	}

	if gridRows < 1 {
		gridRows = 1
	}

	if gridRows > MaxGridRows {
		gridRows = MaxGridRows
	}

	// cellCapacity is how many cells the grid can hold — the loop's stop
	// condition. Both dimensions are clamped to constants above, so it is at
	// most MaxGridCols*MaxGridRows.
	cellCapacity := gridCols * gridRows

	// Grown by append. How many cells the region walk below actually produces
	// depends on how the regions tile the grid, so cellCapacity is a ceiling
	// rather than a count — pre-sizing to it would return trailing nil entries
	// the caller would have to trim.
	var cells []*Cell

	// Calculate region dimensions based on label length
	// Each region is a group of cells sharing the same prefix character(s)
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
						X: xCoordinate + gridRound(cellWidth),
						Y: yCoordinate + gridRound(cellHeight),
					},
				}
				cells = append(cells, cell)
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
		if len(cells) >= cellCapacity {
			break
		}
	}

	return cells
}

// regionShape returns how many columns and rows one region spans, and how many
// distinct region prefixes the label scheme provides.
//
// These must agree with generateCellsWithRegions: it derives the same shape
// from labelLength and bounds its loop by the same prefix count.
func regionShape(numChars, numRowChars, numColChars, labelLength int) (int, int, int) {
	switch labelLength {
	case LabelLength2:
		// One region character, and a region is a single row of columns.
		return numColChars, 1, numChars
	case LabelLength3:
		// One region character spanning a full column-by-row block.
		return numColChars, numRowChars, numChars
	default:
		// Two region characters, so the prefix space is squared.
		return numColChars, numRowChars, numChars * numChars
	}
}

// regionsNeeded returns how many regions it takes to cover a gridCols x
// gridRows grid, counting a clipped edge region as a whole one because that is
// how the fill loop consumes them.
func regionsNeeded(gridCols, gridRows, regionCols, regionRows int) int {
	if regionCols < 1 || regionRows < 1 {
		return 0
	}

	perBand := (gridCols + regionCols - 1) / regionCols
	bands := (gridRows + regionRows - 1) / regionRows

	return perBand * bands
}

// fitToAvailableRegions shrinks the grid until every cell can be reached by an
// available region prefix.
//
// It trims whichever axis currently costs the most regions, so the grid stays
// as close to the screen's aspect ratio as the prefix budget allows, and never
// shrinks below the minimum usable grid.
func fitToAvailableRegions(
	gridCols, gridRows, numChars, numRowChars, numColChars, labelLength int,
) (int, int) {
	regionCols, regionRows, availablePrefixes := regionShape(
		numChars, numRowChars, numColChars, labelLength,
	)

	if regionCols < 1 || regionRows < 1 || availablePrefixes < 1 {
		return gridCols, gridRows
	}

	for regionsNeeded(gridCols, gridRows, regionCols, regionRows) > availablePrefixes {
		canTrimCols := gridCols > MinGridCols
		canTrimRows := gridRows > MinGridRows

		if !canTrimCols && !canTrimRows {
			// Already at the floor; the caller's dimensions are the best
			// available even if some cells go unlabeled.
			break
		}

		// Trim the axis spanning more regions, so the grid degrades evenly
		// rather than collapsing into a stripe.
		colBands := (gridCols + regionCols - 1) / regionCols
		rowBands := (gridRows + regionRows - 1) / regionRows

		switch {
		case canTrimCols && (!canTrimRows || colBands >= rowBands):
			gridCols--
		default:
			gridRows--
		}
	}

	return gridCols, gridRows
}

// calculateLabelLength picks the label length from the cell count and the
// characters available to build labels from.
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

// buildPrefixIndex creates a map of all coordinate prefixes for fast lookup.
func buildPrefixIndex(cells []*Cell) map[string]bool {
	prefixes := make(map[string]bool)
	for _, cell := range cells {
		coord := cell.Coordinate()
		for i := 1; i <= len(coord); i++ {
			prefixes[coord[:i]] = true
		}
	}

	return prefixes
}
