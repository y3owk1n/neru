package grid

import (
	"math"
	"sync"

	"github.com/y3owk1n/neru/internal/domain"
)

// Candidate is a valid grid configuration.
type Candidate struct {
	cols, rows   int
	cellW, cellH int
	score        float64
}

// twoKeyCandidate is a staged two-key layout and the qualities used to rank it.
type twoKeyCandidate struct {
	plan           gridPlan
	score          float64
	cells          int
	twoDimensional bool
}

// planTwoKeyGrid divides both keypresses into 2D stages. The first key selects
// one rectangular region and the second selects a cell inside it, so the final
// column/row counts are the products of the two stages.
//
// Candidates stay within the cell-size planner's target dimensions. Their
// score balances square cells with using more available label pairs, matching
// findValidGridConfigurations without forcing a one-row region that distorts
// the final grid.
func planTwoKeyGrid(
	width, height, targetCols, targetRows, prefixKeys, localKeys int,
) gridPlan {
	prefixStages := keyStageLayouts(prefixKeys, targetCols, targetRows)
	localStages := keyStageLayouts(localKeys, targetCols, targetRows)
	targetCells := targetCols * targetRows
	maxUsableCells := min(int64(targetCells), int64(prefixKeys)*int64(localKeys))

	var best twoKeyCandidate

	found := false

	for _, prefix := range prefixStages {
		for _, local := range localStages {
			cols := prefix.Cols * local.Cols
			rows := prefix.Rows * local.Rows

			cells := cols * rows
			if cols > targetCols || rows > targetRows || cells < MinCharactersLength {
				continue
			}

			cellWidth := float64(width) / float64(cols)
			cellHeight := float64(height) / float64(rows)
			precisionPenalty := float64(maxUsableCells-int64(cells)) /
				float64(maxUsableCells) * ScoreWeight
			candidate := twoKeyCandidate{
				plan: gridPlan{
					dimensions:  domain.GridDimensions{Rows: rows, Cols: cols},
					region:      local,
					labelLength: LabelLength2,
					regionCount: prefixKeys,
				},
				score:          math.Abs(cellWidth/cellHeight-1) + precisionPenalty,
				cells:          cells,
				twoDimensional: cols >= MinGridCols && rows >= MinGridRows,
			}

			if !found || betterTwoKeyCandidate(candidate, best) {
				best = candidate
				found = true
			}
		}
	}

	return best.plan
}

// keyStageLayouts returns every rectangular layout addressable by keyCount,
// bounded by the final grid target so unused labels do not inflate the search.
func keyStageLayouts(keyCount, maxCols, maxRows int) []domain.GridDimensions {
	var layouts []domain.GridDimensions

	for cols := 1; cols <= gridMin(keyCount, maxCols); cols++ {
		rowsLimit := gridMin(keyCount/cols, maxRows)
		for rows := 1; rows <= rowsLimit; rows++ {
			layouts = append(layouts, domain.GridDimensions{Rows: rows, Cols: cols})
		}
	}

	return layouts
}

func betterTwoKeyCandidate(candidate, current twoKeyCandidate) bool {
	switch {
	case candidate.twoDimensional != current.twoDimensional:
		return candidate.twoDimensional
	case candidate.score != current.score:
		return candidate.score < current.score
	default:
		return candidate.cells > current.cells
	}
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

				aspectDiff := math.Abs(cellAspect - 1)

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

// gridRound performs integer division rounding to the nearest integer (half away from zero).
// All coordinates in this package are non-negative so only the positive branch is needed.
func gridRound(numerator int) int {
	return (numerator + CenterDivisor/2) / CenterDivisor
}
