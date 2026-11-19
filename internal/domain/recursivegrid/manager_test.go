package recursivegrid_test

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/recursivegrid"
)

func TestNewManagerDefaultKeys(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()

	manager := recursivegrid.NewManagerWithLayers(
		bounds,
		"", // Empty keys - should use default
		25, 25, 10,
		recursivegrid.DefaultDimensions(),
		nil, nil,
		recursivegrid.SelectionCallbacks{},
		logger,
	)

	assert.Equal(
		t,
		recursivegrid.DefaultKeys,
		manager.Keys(),
		"Should use default keys when empty",
	)
}

func TestManagerReset(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()

	manager := recursivegrid.NewManagerWithLayers(
		bounds,
		"uijk",
		25,
		25,
		10,
		domain.GridDimensions{Rows: 2, Cols: 2},
		nil, nil,
		recursivegrid.SelectionCallbacks{},
		logger,
	)

	manager.HandleInput("u")
	assert.Equal(t, 1, manager.CurrentDepth())

	manager.Reset()
	assert.Equal(t, 0, manager.CurrentDepth(), "Depth should be reset to 0")
	assert.Equal(
		t,
		image.Point{X: 50, Y: 50},
		manager.CurrentCenter(),
		"Center should return to initial",
	)

	// Backtrack steps up one level the way Reset steps up all of them.
	manager.HandleInput("u")

	assert.True(t, manager.Backtrack(), "Backtrack should succeed with history")
	assert.Equal(t, 0, manager.CurrentDepth(), "Depth should be 0 after backtrack")
	assert.Equal(
		t,
		image.Point{X: 50, Y: 50},
		manager.CurrentCenter(),
		"Should return parent center point",
	)
}

func TestManagerHandleInputUnmappedKey(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()

	updateCalled := false
	manager := recursivegrid.NewManagerWithLayers(
		bounds,
		"uijk",
		25,
		25,
		10,
		domain.GridDimensions{Rows: 2, Cols: 2},
		nil, nil,
		recursivegrid.SelectionCallbacks{
			OnUpdate: func(image.Point) { updateCalled = true },
		},
		logger,
	)

	point, completed := manager.HandleInput("z")

	assert.Equal(t, image.Point{}, point, "Should return zero point")
	assert.False(t, completed, "Should not be completed")
	assert.False(t, updateCalled, "Update callback should NOT be called")
}

func TestManagerHandleInputMaxDepth(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()

	completeCalled := false
	manager := recursivegrid.NewManagerWithLayers(
		bounds,
		"uijk",
		1, // minSizeWidth
		1, // minSizeHeight
		2, // maxDepth
		domain.GridDimensions{Rows: 2, Cols: 2},
		nil, nil,
		recursivegrid.SelectionCallbacks{
			OnUpdate:   func(image.Point) {},
			OnComplete: func(image.Point) { completeCalled = true },
		},
		logger,
	)

	// Depth 1
	manager.HandleInput("u")
	assert.Equal(t, 1, manager.CurrentDepth())
	assert.False(t, completeCalled)

	// Depth 2 (Max). Reaching max depth does NOT complete, and the user gets
	// one more selection.
	_, completed := manager.HandleInput("u")
	assert.False(t, completed, "Should NOT complete when reaching max depth")
	assert.False(t, completeCalled, "Complete callback should NOT fire yet")
	assert.Equal(t, 2, manager.CurrentDepth(), "Should be at max depth")

	// Selection AT max depth is the final selection that completes.
	point3, completed3 := manager.HandleInput("k") // Select BottomRight of current
	assert.True(t, completed3, "Should complete on selection at max depth")
	assert.True(t, completeCalled, "Complete callback should fire")
	assert.Equal(t, 2, manager.CurrentDepth(), "Should still be at max depth")

	// Additional input at max depth should still complete
	completeCalled = false
	point4, completed4 := manager.HandleInput("u") // Select TopLeft
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
		10,       // minSizeWidth
		10,       // minSizeHeight
		10,       // maxDepth
		domain.GridDimensions{Rows: 2, Cols: 3},
		nil, nil,
		recursivegrid.SelectionCallbacks{
			OnUpdate: func(image.Point) { updateCalled = true },
		},
		logger,
	)
	assert.Equal(t, 3, manager.GridCols())
	assert.Equal(t, 2, manager.GridRows())
	assert.Equal(t, "gcrhtn", manager.Keys())
	// Select cell 'g' (index 0, top-left) -> (0,0)-(40,50), center (20,25)
	point, completed := manager.HandleInput("g")
	assert.Equal(t, image.Point{X: 20, Y: 25}, point)
	assert.False(t, completed)
	assert.True(t, updateCalled)
	assert.Equal(t, 1, manager.CurrentDepth())
}

func TestHandleInput_InvalidKeyLength_FallsBackToDefault(t *testing.T) {
	// "€a" is 4 bytes but 2 runes. A 2x2 grid needs 4 keys, so only a count
	// in runes rejects it.
	manager := recursivegrid.NewManagerWithLayers(
		image.Rect(0, 0, 100, 100),
		"€a",
		25, 25, 10,
		domain.GridDimensions{Rows: 2, Cols: 2},
		nil, nil,
		recursivegrid.SelectionCallbacks{},
		zap.NewNop(),
	)
	assert.Equal(
		t,
		recursivegrid.DefaultKeys,
		manager.Keys(),
		"Should fall back to default keys when the key count in runes is wrong",
	)
}

// TestNewManagerWithLayers_MismatchedDepthKeys_DropsOrphan covers every way a
// per-depth override can fail to line up. Each is dropped, so depth 0 keeps
// the base keys and shape.
func TestNewManagerWithLayers_MismatchedDepthKeys_DropsOrphan(t *testing.T) {
	testCases := []struct {
		name         string
		depthLayouts map[int]recursivegrid.DepthLayout
		depthKeys    map[int]string
	}{
		{
			name:         "keys with no layout",
			depthLayouts: map[int]recursivegrid.DepthLayout{},
			depthKeys:    map[int]string{0: "qwerasdf"},
		},
		{
			name:         "layout with no keys",
			depthLayouts: map[int]recursivegrid.DepthLayout{0: {GridCols: 3, GridRows: 3}},
			depthKeys:    map[int]string{},
		},
		{
			name:         "key count does not match the layout",
			depthLayouts: map[int]recursivegrid.DepthLayout{0: {GridCols: 3, GridRows: 3}},
			depthKeys:    map[int]string{0: "abcd"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			manager := recursivegrid.NewManagerWithLayers(
				image.Rect(0, 0, 120, 100),
				"uijk",
				10, 10, 10,
				domain.GridDimensions{Rows: 2, Cols: 2},
				testCase.depthLayouts,
				testCase.depthKeys,
				recursivegrid.SelectionCallbacks{},
				zap.NewNop(),
			)
			assert.Equal(t, "uijk", manager.Keys())
			assert.Equal(t, 2, manager.GridCols())
			assert.Equal(t, 2, manager.GridRows())
		})
	}
}

// TestNewManagerWithLayers_NilLogger_DoesNotPanic covers every path that logs.
// Constructors in this tree accept a nil logger and fall back to a no-op.
func TestNewManagerWithLayers_NilLogger_DoesNotPanic(t *testing.T) {
	const twoByTwoKeys, layerKeys = "uijk", "abcd"

	tests := []struct {
		name         string
		keys         string
		dims         domain.GridDimensions
		depthLayouts map[int]recursivegrid.DepthLayout
		depthKeys    map[int]string
	}{
		{
			name: "invalid dimensions",
			keys: twoByTwoKeys,
			dims: domain.GridDimensions{Rows: 0, Cols: 3},
		},
		{name: "key count mismatch", keys: "abc", dims: domain.GridDimensions{Rows: 2, Cols: 2}},
		{
			name:         "layout without keys",
			keys:         twoByTwoKeys,
			dims:         domain.GridDimensions{Rows: 2, Cols: 2},
			depthLayouts: map[int]recursivegrid.DepthLayout{1: {GridCols: 2, GridRows: 2}},
		},
		{
			name:         "layer key count mismatch",
			keys:         twoByTwoKeys,
			dims:         domain.GridDimensions{Rows: 2, Cols: 2},
			depthLayouts: map[int]recursivegrid.DepthLayout{1: {GridCols: 3, GridRows: 3}},
			depthKeys:    map[int]string{1: layerKeys},
		},
		{
			name:      "keys without layout",
			keys:      twoByTwoKeys,
			dims:      domain.GridDimensions{Rows: 2, Cols: 2},
			depthKeys: map[int]string{1: layerKeys},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				manager := recursivegrid.NewManagerWithLayers(
					image.Rect(0, 0, 120, 100),
					testCase.keys,
					10, 10, 10,
					testCase.dims,
					testCase.depthLayouts,
					testCase.depthKeys,
					recursivegrid.SelectionCallbacks{},
					nil,
				)
				manager.HandleInput("?")
				manager.HandleInput(manager.Keys()[:1])
				manager.MoveDirection(domain.DirectionRight, 1)
			})
		})
	}
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
		10, 10, 10,
		domain.GridDimensions{Rows: 2, Cols: 2},
		depthLayouts,
		depthKeys,
		recursivegrid.SelectionCallbacks{},
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

func TestManagerHandleInput_KeyToCellLiteralSpaceKey(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()
	// Use a key mapping that contains a literal space character (" ") as one of the 4 keys.
	manager := recursivegrid.NewManagerWithLayers(
		bounds,
		"ui k", // keys: u=0, i=1, ' '=2, k=3
		25,
		25,
		10,
		domain.GridDimensions{Rows: 2, Cols: 2},
		nil, nil,
		recursivegrid.SelectionCallbacks{OnUpdate: func(image.Point) {}},
		logger,
	)
	// Pressing the literal space should map to cell index 2 (bottom-left).
	point, completed := manager.HandleInput(" ")
	assert.False(t, completed, "Should not be completed")
	// Bottom-left cell of 100x100 with 2x2 grid: (0,50)-(50,100), center (25,75)
	assert.Equal(t, image.Point{X: 25, Y: 75}, point,
		"Literal space key should map in key mapping")
	assert.Equal(t, 1, manager.CurrentDepth(), "Depth should advance to 1")
}

func TestManagerHandleInput_KeyToCellNormalizesFullwidthChars(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()
	manager := recursivegrid.NewManagerWithLayers(
		bounds,
		"uijk",
		25,
		25,
		10,
		domain.GridDimensions{Rows: 2, Cols: 2},
		nil, nil,
		recursivegrid.SelectionCallbacks{OnUpdate: func(image.Point) {}},
		logger,
	)
	// Fullwidth 'u' (U+FF55) should normalize to halfwidth 'u' and match cell 0.
	point, completed := manager.HandleInput("\uFF55") // fullwidth u
	assert.False(t, completed, "Should not be completed")
	// Top-left cell of 100x100 with 2x2 grid: (0,0)-(50,50), center (25,25)
	assert.Equal(t, image.Point{X: 25, Y: 25}, point,
		"Fullwidth 'ｕ' should match halfwidth 'u' in key mapping")
	assert.Equal(t, 1, manager.CurrentDepth(), "Depth should advance to 1")
}

func TestHandleInput_ValidMultibyteKeys(t *testing.T) {
	keys := "€abc" // 4 runes, valid length
	logger := zap.NewNop()
	screenBounds := image.Rect(0, 0, 100, 100)
	manager := recursivegrid.NewManagerWithLayers(
		screenBounds,
		keys,
		25,
		25,
		10,
		domain.GridDimensions{Rows: 2, Cols: 2},
		nil, nil,
		recursivegrid.SelectionCallbacks{},
		logger,
	)
	assert.Equal(
		t,
		"€abc",
		manager.Keys(),
		"Should accept valid multibyte keys",
	)
	assert.NotPanics(t, func() {
		// € is the first key, so it should map to Cell 0 (TopLeft)
		center, _ := manager.HandleInput("€")
		// TopLeft of 100x100 is 0,0 to 50,50. Center is 25,25.
		assert.Equal(t, image.Point{X: 25, Y: 25}, center)
	}, "HandleInput should handle multibyte key input")
}
