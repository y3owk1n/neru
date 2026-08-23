package grid

import (
	"image"
	"strings"

	"go.uber.org/zap"
)

// generateCellsWithRegions lays cells out region by region, left to right and
// top to bottom, so labels sharing a prefix stay spatially grouped.
func generateCellsWithRegions(
	chars, rowChars, colChars []rune,
	plan gridPlan,
	bounds image.Rectangle,
	baseCellWidth, baseCellHeight, remainderWidth, remainderHeight int,
	logger *zap.Logger,
) []*Cell {
	numChars := len(chars)
	gridCols := plan.dimensions.Cols
	gridRows := plan.dimensions.Rows

	logger.Debug("Generating cells with regions",
		zap.Int("num_chars", numChars),
		zap.Int("grid_cols", gridCols),
		zap.Int("grid_rows", gridRows),
		zap.Int("label_length", plan.labelLength))

	// Clamp both dimensions into [1, Max]. Dimensions arrive as parameters, so
	// a zero or negative count would otherwise reach the arithmetic below, and
	// the constant bounds are what keep the cell count allocation-safe.
	// Explicit comparisons, not min/max: static analysis follows this form when
	// proving the allocation bound.
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

	// The loop's stop condition. A ceiling, not a count: how many cells the
	// region walk produces depends on how regions tile the grid, so cells is
	// grown by append rather than pre-sized.
	cellCapacity := gridCols * gridRows

	var cells []*Cell

	xStarts, yStarts := cellEdges(bounds, gridCols, gridRows,
		baseCellWidth, baseCellHeight, remainderWidth, remainderHeight)

	currentCol := 0
	currentRow := 0
	regionIndex := 0

	for regionIndex < plan.regionCount && currentRow < gridRows {
		regionChar1, regionChar2 := regionPrefix(chars, numChars, regionIndex, plan.labelLength)

		colsForRegion := gridMin(plan.region.Cols, gridCols-currentCol)
		rowsForRegion := gridMin(plan.region.Rows, gridRows-currentRow)

		for rowIndex := range rowsForRegion {
			for colIndex := range colsForRegion {
				globalCol := currentCol + colIndex
				globalRow := currentRow + rowIndex

				if globalCol >= gridCols || globalRow >= gridRows {
					break
				}

				coordinate := cellCoordinate(
					plan.labelLength, regionChar1, regionChar2,
					colChars, rowChars, plan.region.Cols, colIndex, rowIndex,
				)

				// Distribute the remainder pixels across the leading cells.
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

				cells = append(cells, &Cell{
					coordinate: coordinate,
					bounds: image.Rect(
						xCoordinate, yCoordinate,
						xCoordinate+cellWidth, yCoordinate+cellHeight,
					),
					center: image.Point{
						X: xCoordinate + gridRound(cellWidth),
						Y: yCoordinate + gridRound(cellHeight),
					},
				})
			}
		}

		currentCol += colsForRegion
		if currentCol >= gridCols {
			currentCol = 0
			currentRow += rowsForRegion
		}

		regionIndex++

		if len(cells) >= cellCapacity {
			break
		}
	}

	return cells
}

// regionPrefix returns the one or two characters naming a region.
func regionPrefix(chars []rune, numChars, regionIndex, labelLength int) (rune, rune) {
	if labelLength == LabelLength2 || labelLength == LabelLength3 {
		return chars[regionIndex%numChars], 0
	}

	return chars[regionIndex/numChars%numChars], chars[regionIndex%numChars]
}

// cellCoordinate builds one cell's label. A two-key region uses its second key
// as a row-major local-cell index; longer labels retain their column and row
// characters after the region prefix.
func cellCoordinate(
	labelLength int,
	regionChar1, regionChar2 rune,
	colChars, rowChars []rune,
	regionCols, colIndex, rowIndex int,
) string {
	var stringBuilder strings.Builder

	switch labelLength {
	case LabelLength2:
		stringBuilder.Grow(StringBuilderGrow2)
		stringBuilder.WriteRune(regionChar1)

		localIndex := rowIndex*regionCols + colIndex
		stringBuilder.WriteRune(colChars[localIndex%len(colChars)])
	case LabelLength3:
		stringBuilder.Grow(StringBuilderGrow3)
		stringBuilder.WriteRune(regionChar1)
		stringBuilder.WriteRune(colChars[colIndex%len(colChars)])
		stringBuilder.WriteRune(rowChars[rowIndex%len(rowChars)])
	default:
		stringBuilder.Grow(StringBuilderGrow4)
		stringBuilder.WriteRune(regionChar1)
		stringBuilder.WriteRune(regionChar2)
		stringBuilder.WriteRune(colChars[colIndex%len(colChars)])
		stringBuilder.WriteRune(rowChars[rowIndex%len(rowChars)])
	}

	return stringBuilder.String()
}

// cellEdges precomputes each column's left edge and each row's top edge, with
// the remainder pixels spread across the leading cells.
func cellEdges(
	bounds image.Rectangle,
	gridCols, gridRows,
	baseCellWidth, baseCellHeight, remainderWidth, remainderHeight int,
) ([]int, []int) {
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

	return xStarts, yStarts
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
	gridCols, gridRows, regionCols, regionRows, availablePrefixes int,
) (int, int) {
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
