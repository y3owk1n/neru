package grid

import (
	"image"
	"testing"

	"go.uber.org/zap"
)

// This is a whitebox test — it calls the unexported generator directly, which
// the _internal_test.go suffix permits.
//
// generateCellsWithRegions takes its dimensions as parameters rather than
// deriving them, so it cannot assume a caller clamped them. Before the clamp it
// now applies, a zero row count divided by zero in the overflow guard and a
// negative one reached make() with a negative length — both panics, in a
// function on the grid-build path.
//
// CodeQL flagged the allocation; these are the inputs that make the flag real.
func TestGenerateCellsWithRegionsSurvivesDegenerateDimensions(t *testing.T) {
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
				len(chars), testCase.gridCols, testCase.gridRows, LabelLength2,
				bounds,
				10, 10, 0, 0,
				zap.NewNop(),
			)

			// Reaching here at all is most of the point — every one of these
			// inputs panicked before the clamp. The assertions pin the two
			// things the clamp is supposed to guarantee: the result never
			// exceeds what the clamped grid can hold, and it never contains a
			// nil cell. The exact count is deliberately not pinned; what a
			// degenerate grid should contain is not what this is about.
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
