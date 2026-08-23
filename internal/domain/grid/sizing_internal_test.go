package grid

import (
	"math"
	"testing"
)

// TestPlanTwoKeyGrid_KeepsCellsNearSquareAcrossScreenShapes pins the reason
// the two-stage planner exists: a two-key cap must not collapse a wide or tall
// display into one-row prefix bands. Each stage stays within its own key budget
// and the combined layout stays inside the ordinary cell-size planner's target.
func TestPlanTwoKeyGrid_KeepsCellsNearSquareAcrossScreenShapes(t *testing.T) {
	tests := []struct {
		name                   string
		width, height          int
		targetCols, targetRows int
	}{
		{name: "widescreen", width: 1920, height: 1080, targetCols: 64, targetRows: 36},
		{name: "four by three", width: 1280, height: 960, targetCols: 42, targetRows: 32},
		{name: "ultrawide", width: 5120, height: 1440, targetCols: 102, targetRows: 28},
		{name: "portrait", width: 1080, height: 1920, targetCols: 36, targetRows: 64},
	}

	const keys = 25

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			plan := planTwoKeyGrid(
				testCase.width,
				testCase.height,
				testCase.targetCols,
				testCase.targetRows,
				keys,
				keys,
			)
			dimensions := plan.dimensions
			region := plan.region

			cellAspect := (float64(testCase.width) / float64(dimensions.Cols)) /
				(float64(testCase.height) / float64(dimensions.Rows))
			if diff := math.Abs(cellAspect - 1); diff > 0.1 {
				t.Errorf("cell aspect ratio = %.3f, want within 10%% of square", cellAspect)
			}

			if dimensions.Cols > testCase.targetCols || dimensions.Rows > testCase.targetRows {
				t.Errorf(
					"plan = %dx%d, outside target %dx%d",
					dimensions.Cols,
					dimensions.Rows,
					testCase.targetCols,
					testCase.targetRows,
				)
			}

			if localCells := region.CellCount(); localCells > keys {
				t.Errorf("local region uses %d keys, only %d available", localCells, keys)
			}

			if dimensions.Cols%region.Cols != 0 || dimensions.Rows%region.Rows != 0 {
				t.Fatalf(
					"plan %dx%d is not tiled by its %dx%d regions",
					dimensions.Cols,
					dimensions.Rows,
					region.Cols,
					region.Rows,
				)
			}

			regions := dimensions.Cols / region.Cols * (dimensions.Rows / region.Rows)
			if regions > keys {
				t.Errorf("plan uses %d prefix regions, only %d keys available", regions, keys)
			}
		})
	}
}
