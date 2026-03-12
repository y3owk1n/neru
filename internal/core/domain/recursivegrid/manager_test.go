package recursivegrid_test

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain/recursivegrid"
)

func TestNewManager(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()

	manager := recursivegrid.NewManager(
		bounds,
		"uijk",
		",",
		"",
		[]string{"escape"},
		func() {},
		func(point image.Point) {},
		logger,
	)

	assert.NotNil(t, manager, "Manager should not be nil")
	assert.Equal(t, "uijk", manager.Keys(), "Keys should be set")
}

func TestNewManagerDefaultKeys(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()

	manager := recursivegrid.NewManager(
		bounds,
		"", // Empty keys - should use default
		",",
		"",
		[]string{"escape"},
		nil,
		nil,
		logger,
	)

	assert.Equal(
		t,
		recursivegrid.DefaultKeys,
		manager.Keys(),
		"Should use default keys when empty",
	)
}

func TestManagerHandleInputCellSelection(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()

	updateCalled := false
	manager := recursivegrid.NewManager(
		bounds,
		"uijk",
		",",
		"",
		[]string{"escape"},
		func() { updateCalled = true },
		nil,
		logger,
	)

	// Select top-left cell (key 'u')
	point, completed, shouldExit := manager.HandleInput("u")

	assert.Equal(t, image.Point{X: 25, Y: 25}, point, "Should return center of top-left cell")
	assert.False(t, completed, "Should not be completed")
	assert.False(t, shouldExit, "Should not exit")
	assert.True(t, updateCalled, "Update callback should be called")
	assert.Equal(t, 1, manager.CurrentDepth(), "Depth should be 1")
}

func TestManagerHandleInputExitKey(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()

	manager := recursivegrid.NewManager(
		bounds,
		"uijk",
		",",
		"",
		[]string{"escape"},
		nil,
		nil,
		logger,
	)

	point, completed, shouldExit := manager.HandleInput("escape")

	assert.Equal(t, image.Point{}, point, "Should return zero point")
	assert.False(t, completed, "Should not be completed")
	assert.True(t, shouldExit, "Should exit on escape key")
}

func TestManagerHandleInputResetKey(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()

	updateCalled := false
	manager := recursivegrid.NewManager(
		bounds,
		"uijk",
		",",
		"",
		[]string{"escape"},
		func() { updateCalled = true },
		nil,
		logger,
	)

	manager.HandleInput("u")
	assert.Equal(t, 1, manager.CurrentDepth())

	point, completed, shouldExit := manager.HandleInput(",")

	assert.NotEqual(t, image.Point{}, point, "Should return center point")
	assert.Equal(t, image.Point{X: 50, Y: 50}, point, "Should return initial center point")
	assert.False(t, completed, "Should not be completed")
	assert.False(t, shouldExit, "Should not exit")
	assert.True(t, updateCalled, "Update callback should be called")
	assert.Equal(t, 0, manager.CurrentDepth(), "Depth should be reset to 0")
}

func TestManagerHandleInputResetKeyEmptyFallbackToSpace(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()
	updateCalled := false
	manager := recursivegrid.NewManager(
		bounds,
		"uijk",
		"", // Empty reset key — should fall back to space
		"",
		[]string{"escape"},
		func() { updateCalled = true },
		nil,
		logger,
	)
	manager.HandleInput("u")
	assert.Equal(t, 1, manager.CurrentDepth())
	// Space should trigger reset via the empty-string-to-space fallback
	point, completed, shouldExit := manager.HandleInput(" ")

	assert.NotEqual(t, image.Point{}, point, "Should return center point")
	assert.Equal(t, image.Point{X: 50, Y: 50}, point, "Should return initial center point")
	assert.False(t, completed, "Should not be completed")
	assert.False(t, shouldExit, "Should not exit")
	assert.True(t, updateCalled, "Update callback should be called")
	assert.Equal(t, 0, manager.CurrentDepth(), "Depth should be reset to 0")
}

func TestManagerHandleInputBacktrack(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()

	updateCalled := false
	manager := recursivegrid.NewManager(
		bounds,
		"uijk",
		",",
		"",
		[]string{"escape"},
		func() { updateCalled = true },
		nil,
		logger,
	)

	manager.HandleInput("u")
	assert.Equal(t, 1, manager.CurrentDepth())

	// Reset update flag
	updateCalled = false

	point, completed, shouldExit := manager.HandleInput("backspace")

	assert.NotEqual(t, image.Point{}, point, "Should return center point")
	assert.Equal(t, image.Point{X: 50, Y: 50}, point, "Should return parent center point")
	assert.False(t, completed, "Should not be completed")
	assert.False(t, shouldExit, "Should not exit")
	assert.True(t, updateCalled, "Update callback should be called")
	assert.Equal(t, 0, manager.CurrentDepth(), "Depth should be 0 after backtrack")
}

func TestManagerHandleInputUnmappedKey(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()

	updateCalled := false
	manager := recursivegrid.NewManager(
		bounds,
		"uijk",
		",",
		"",
		[]string{"escape"},
		func() { updateCalled = true },
		nil,
		logger,
	)

	point, completed, shouldExit := manager.HandleInput("z")

	assert.Equal(t, image.Point{}, point, "Should return zero point")
	assert.False(t, completed, "Should not be completed")
	assert.False(t, shouldExit, "Should not exit")
	assert.False(t, updateCalled, "Update callback should NOT be called")
}

func TestManagerHandleInputCompletion(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()

	completeCalled := false

	var completePoint image.Point

	manager := recursivegrid.NewManagerWithLayers(
		bounds,
		"uijk",
		",",
		"",
		[]string{"escape"},
		50, // minSizeWidth large enough to complete quickly
		50, // minSizeHeight
		10,
		2, // gridCols 2
		2, // gridRows 2
		nil, nil,
		nil,
		func(p image.Point) {
			completeCalled = true
			completePoint = p
		},
		logger,
	)

	// Initial size 100x100
	// Select u -> 50x50 (top-left), depth becomes 1
	// Since minSize is 50, 50/2 = 25 < 50. CanDivide is false.
	// But completion only triggers on the NEXT key press (at the final depth).

	point, completed, shouldExit := manager.HandleInput("u")

	assert.False(
		t,
		completed,
		"Should NOT be completed yet — user must make one more selection at final depth",
	)
	assert.False(t, shouldExit, "Should not exit")
	assert.False(t, completeCalled, "Complete callback should NOT be called yet")
	assert.Equal(t, image.Point{X: 25, Y: 25}, point, "Should return center of top-left cell")

	// Now at final depth (CanDivide is false), select a sub-cell to complete
	point2, completed2, shouldExit2 := manager.HandleInput("k") // BottomRight

	assert.True(t, completed2, "Should be completed after selection at final depth")
	assert.False(t, shouldExit2, "Should not exit")
	assert.True(t, completeCalled, "Complete callback should be called")
	assert.Equal(t, point2, completePoint, "Complete point should match return point")
}

func TestManagerHandleInputMaxDepth(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()

	completeCalled := false
	manager := recursivegrid.NewManagerWithLayers(
		bounds,
		"uijk",
		",",
		"",
		[]string{"escape"},
		1, // minSizeWidth
		1, // minSizeHeight
		2, // maxDepth
		2, // gridCols 2
		2, // gridRows 2
		nil, nil,
		func() {},
		func(point image.Point) { completeCalled = true },
		logger,
	)

	// Depth 1
	manager.HandleInput("u")
	assert.Equal(t, 1, manager.CurrentDepth())
	assert.False(t, completeCalled)

	// Depth 2 (Max) — reaching max depth does NOT complete; user gets one more selection
	_, completed, _ := manager.HandleInput("u")
	assert.False(t, completed, "Should NOT complete when reaching max depth")
	assert.False(t, completeCalled, "Complete callback should NOT fire yet")
	assert.Equal(t, 2, manager.CurrentDepth(), "Should be at max depth")

	// Selection AT max depth — this is the final selection that completes
	point3, completed3, _ := manager.HandleInput("k") // Select BottomRight of current
	assert.True(t, completed3, "Should complete on selection at max depth")
	assert.True(t, completeCalled, "Complete callback should fire")
	assert.Equal(t, 2, manager.CurrentDepth(), "Should still be at max depth")

	// Additional input at max depth should still complete
	completeCalled = false
	point4, completed4, _ := manager.HandleInput("u") // Select TopLeft
	assert.True(t, completed4)
	assert.Equal(t, 2, manager.CurrentDepth(), "Should still be at max depth")
	assert.NotEqual(t, point3, point4, "Should return different point (different sub-cell center)")
}

func TestManagerWithLayers_NonSquare3x2(t *testing.T) {
	bounds := image.Rect(0, 0, 120, 100)
	logger := zap.NewNop()
	updateCalled := false
	manager := recursivegrid.NewManagerWithLayers(
		bounds,
		"gcrhtn", // 6 keys for 3x2
		",",
		"",
		[]string{"escape"},
		10, // minSizeWidth
		10, // minSizeHeight
		10, // maxDepth
		3,  // gridCols
		2,  // gridRows
		nil, nil,
		func() { updateCalled = true },
		nil,
		logger,
	)
	assert.Equal(t, 3, manager.GridCols())
	assert.Equal(t, 2, manager.GridRows())
	assert.Equal(t, "gcrhtn", manager.Keys())
	// Select cell 'g' (index 0, top-left) -> (0,0)-(40,50), center (20,25)
	point, completed, shouldExit := manager.HandleInput("g")
	assert.Equal(t, image.Point{X: 20, Y: 25}, point)
	assert.False(t, completed)
	assert.False(t, shouldExit)
	assert.True(t, updateCalled)
	assert.Equal(t, 1, manager.CurrentDepth())
}

func TestManagerWithLayers_InvalidColsOnly_FallsBack(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()
	// gridCols=1 is invalid, gridRows=3 is valid
	// Manager corrects gridCols to 2, then key count = 2*3 = 6
	// "uijk" has 4 keys ≠ 6, so it falls back to default keys "uijk" with 2x2
	manager := recursivegrid.NewManagerWithLayers(
		bounds,
		"uijk",
		",",
		"",
		[]string{"escape"},
		10,
		10,
		10,
		1, // invalid gridCols
		3, // valid gridRows
		nil, nil,
		nil,
		nil,
		logger,
	)
	// After fallback, should use default 2x2 with default keys
	assert.Equal(t, 2, manager.GridCols())
	assert.Equal(t, 2, manager.GridRows())
	assert.Equal(t, recursivegrid.DefaultKeys, manager.Keys())
}

func TestManagerWithLayers_InvalidRowsOnly_FallsBack(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()
	// gridCols=3 is valid, gridRows=0 is invalid
	// Manager corrects gridRows to 2, then key count = 3*2 = 6
	// "uijk" has 4 keys ≠ 6, so it falls back to default keys "uijk" with 2x2
	manager := recursivegrid.NewManagerWithLayers(
		bounds,
		"uijk",
		",",
		"",
		[]string{"escape"},
		10,
		10,
		10,
		3, // valid gridCols
		0, // invalid gridRows
		nil, nil,
		nil,
		nil,
		logger,
	)
	assert.Equal(t, 2, manager.GridCols())
	assert.Equal(t, 2, manager.GridRows())
	assert.Equal(t, recursivegrid.DefaultKeys, manager.Keys())
}

func TestHandleInput_InvalidKeyLength_FallsBackToDefault(t *testing.T) {
	keys := "€ab" // 3 runes, invalid length
	logger := zap.NewNop()
	screenBounds := image.Rect(0, 0, 100, 100)
	manager := recursivegrid.NewManager(
		screenBounds,
		keys,
		",",
		"",
		[]string{"escape"},
		nil,
		nil,
		logger,
	)
	assert.Equal(
		t,
		recursivegrid.DefaultKeys,
		manager.Keys(),
		"Should fall back to default keys when given invalid length keys (even with multibyte)",
	)
	assert.NotPanics(t, func() {
		_, _, shouldExit := manager.HandleInput("c")
		assert.False(t, shouldExit, "Should not exit")
	}, "HandleInput should not panic after fallback")
}

func TestNewManagerWithLayers_MismatchedDepthKeys_DropsOrphan(t *testing.T) {
	bounds := image.Rect(0, 0, 120, 100)
	logger := zap.NewNop()
	// depthKeys has depth 0, but depthLayouts does not → depth 0 override should be dropped
	depthLayouts := map[int]recursivegrid.DepthLayout{}
	depthKeys := map[int]string{
		0: "qwerasdf", // 8 keys, but no layout for depth 0
	}
	manager := recursivegrid.NewManagerWithLayers(
		bounds,
		"uijk",
		",",
		"",
		[]string{"escape"},
		10, 10, 10,
		2, 2,
		depthLayouts,
		depthKeys,
		nil, nil,
		logger,
	)
	// At depth 0, should use default keys since the orphan override was dropped
	assert.Equal(t, "uijk", manager.Keys())
	assert.Equal(t, 2, manager.GridCols())
	assert.Equal(t, 2, manager.GridRows())
}

func TestNewManagerWithLayers_MismatchedDepthLayouts_DropsOrphan(t *testing.T) {
	bounds := image.Rect(0, 0, 120, 100)
	logger := zap.NewNop()
	// depthLayouts has depth 0, but depthKeys does not → depth 0 override should be dropped
	depthLayouts := map[int]recursivegrid.DepthLayout{
		0: {GridCols: 3, GridRows: 3},
	}
	depthKeys := map[int]string{}
	manager := recursivegrid.NewManagerWithLayers(
		bounds,
		"uijk",
		",",
		"",
		[]string{"escape"},
		10, 10, 10,
		2, 2,
		depthLayouts,
		depthKeys,
		nil, nil,
		logger,
	)
	// At depth 0, should use defaults since the orphan layout override was dropped
	assert.Equal(t, "uijk", manager.Keys())
	assert.Equal(t, 2, manager.GridCols())
	assert.Equal(t, 2, manager.GridRows())
}

func TestNewManagerWithLayers_KeyCountMismatch_DropsOverride(t *testing.T) {
	bounds := image.Rect(0, 0, 120, 100)
	logger := zap.NewNop()
	// depthLayouts says 3x3 (9 cells), but depthKeys only has 4 keys → mismatch
	depthLayouts := map[int]recursivegrid.DepthLayout{
		0: {GridCols: 3, GridRows: 3},
	}
	depthKeys := map[int]string{
		0: "abcd", // 4 keys ≠ 9
	}
	manager := recursivegrid.NewManagerWithLayers(
		bounds,
		"uijk",
		",",
		"",
		[]string{"escape"},
		10, 10, 10,
		2, 2,
		depthLayouts,
		depthKeys,
		nil, nil,
		logger,
	)
	// At depth 0, should use defaults since the mismatched override was dropped
	assert.Equal(t, "uijk", manager.Keys())
	assert.Equal(t, 2, manager.GridCols())
	assert.Equal(t, 2, manager.GridRows())
}

func TestNewManagerWithLayers_ConsistentOverride_Works(t *testing.T) {
	bounds := image.Rect(0, 0, 120, 100)
	logger := zap.NewNop()
	depthLayouts := map[int]recursivegrid.DepthLayout{
		0: {GridCols: 3, GridRows: 3},
	}
	depthKeys := map[int]string{
		0: "qweasdzxc", // 9 keys for 3x3
	}
	manager := recursivegrid.NewManagerWithLayers(
		bounds,
		"uijk",
		",",
		"",
		[]string{"escape"},
		10, 10, 10,
		2, 2,
		depthLayouts,
		depthKeys,
		nil, nil,
		logger,
	)
	// At depth 0, should use the override
	assert.Equal(t, "qweasdzxc", manager.Keys())
	assert.Equal(t, 3, manager.GridCols())
	assert.Equal(t, 3, manager.GridRows())
	// After selecting a cell and moving to depth 1, should use defaults
	manager.HandleInput("q")
	assert.Equal(t, 1, manager.CurrentDepth())
	assert.Equal(t, "uijk", manager.Keys())
	assert.Equal(t, 2, manager.GridCols())
	assert.Equal(t, 2, manager.GridRows())
}

func TestHandleInput_ValidMultibyteKeys(t *testing.T) {
	keys := "€abc" // 4 runes, valid length
	logger := zap.NewNop()
	screenBounds := image.Rect(0, 0, 100, 100)
	manager := recursivegrid.NewManager(
		screenBounds,
		keys,
		",",
		"",
		[]string{"escape"},
		nil,
		nil,
		logger,
	)
	assert.Equal(
		t,
		"€abc",
		manager.Keys(),
		"Should accept valid multibyte keys",
	)
	assert.NotPanics(t, func() {
		// Test handling a multibyte key input
		// € is the first key, so it should map to Cell 0 (TopLeft)
		center, _, _ := manager.HandleInput("€")
		// TopLeft of 100x100 is 0,0 to 50,50. Center is 25,25.
		assert.Equal(t, image.Point{X: 25, Y: 25}, center)
	}, "HandleInput should handle multibyte key input")
}
