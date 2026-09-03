package grid

import (
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/domain"
)

// This is a whitebox test — it calls the unexported generator directly, which
// the _internal_test.go suffix permits.
//
// generateCellsWithRegions takes its dimensions as parameters rather than
// deriving them, so a caller can hand it anything. These are the inputs that
// would reach the arithmetic and the allocation unbounded if the clamp were
// not there: zero, negative, and larger than the grid can hold.
func TestGenerateCellsWithRegions_SurvivesDegenerateDimensions(t *testing.T) {
	tests := []struct {
		name     string
		gridCols int
		gridRows int
	}{
		{name: "zero rows", gridCols: 4, gridRows: 0},
		{name: "zero cols", gridCols: 0, gridRows: 4},
		{name: "both zero", gridCols: 0, gridRows: 0},
		{name: "negative rows", gridCols: 4, gridRows: -1},
		{name: "negative cols", gridCols: -1, gridRows: 4},
		{name: "both negative", gridCols: -8, gridRows: -8},
		{name: "absurdly large", gridCols: MaxGridCols * 100, gridRows: MaxGridRows * 100},
	}

	chars := []rune("abcdefghijklmnopqrstuvwxyz")
	bounds := image.Rect(0, 0, 1920, 1080)

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cells := generateCellsWithRegions(
				chars, chars, chars,
				gridPlan{
					dimensions: domain.GridDimensions{
						Rows: testCase.gridRows,
						Cols: testCase.gridCols,
					},
					region:      domain.GridDimensions{Rows: 1, Cols: len(chars)},
					labelLength: LabelLength2,
					regionCount: len(chars),
				},
				bounds,
				10, 10, 0, 0,
				zap.NewNop(),
			)

			// Returning at all is most of the point: each of these inputs
			// reaches arithmetic and an allocation that only the clamp keeps
			// in range. The assertions pin what the clamp guarantees — a
			// result no larger than the clamped grid can hold, and no nil
			// cells. The exact count is left unpinned, since what a degenerate
			// grid should contain is not what this covers.
			if len(cells) > MaxGridCols*MaxGridRows {
				t.Errorf("returned %d cells, more than the clamped maximum", len(cells))
			}

			for i, cell := range cells {
				if cell == nil {
					t.Errorf("cell %d is nil", i)

					break
				}
			}
		})
	}
}
