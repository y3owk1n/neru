package recursivegrid_test

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/y3owk1n/neru/internal/core/domain/recursivegrid"
	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()

	manager := recursivegrid.NewManager(
		bounds,
		"uijk",
		",",
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

func TestManagerHandleInputBacktrack(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()

	updateCalled := false
	manager := recursivegrid.NewManager(
		bounds,
		"uijk",
		",",
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

	manager := recursivegrid.NewManagerWithConfig(
		bounds,
		"uijk",
		",",
		[]string{"escape"},
		50, // minSize large enough to complete quickly
		10,
		2, // gridSize 2x2
		nil,
		func(p image.Point) {
			completeCalled = true
			completePoint = p
		},
		logger,
	)

	// Initial size 100x100
	// Select u -> 50x50 (top-left)
	// Since minSize is 50, 50x50 is >= 50. Wait, CanDivide checks halfWidth >= minSize.
	// 50/2 = 25. 25 < 50. So CanDivide should be false.

	point, completed, shouldExit := manager.HandleInput("u")

	assert.True(t, completed, "Should be completed")
	assert.False(t, shouldExit, "Should not exit")
	assert.True(t, completeCalled, "Complete callback should be called")
	assert.Equal(t, point, completePoint, "Complete point should match return point")
}

func TestManagerHandleInputMaxDepth(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()

	completeCalled := false
	manager := recursivegrid.NewManagerWithConfig(
		bounds,
		"uijk",
		",",
		[]string{"escape"},
		1, // minSize
		2, // maxDepth
		2, // gridSize 2x2
		func() {},
		func(point image.Point) { completeCalled = true },
		logger,
	)

	// Depth 1
	manager.HandleInput("u")
	assert.Equal(t, 1, manager.CurrentDepth())
	assert.False(t, completeCalled)

	// Depth 2 (Max)
	point2, completed, _ := manager.HandleInput("u")
	assert.True(t, completed)
	assert.True(t, completeCalled)
	assert.Equal(t, 2, manager.CurrentDepth(), "Should stay at max depth")

	// Input at Max Depth
	// Should return sub-cell center but NOT change depth
	point3, completed3, _ := manager.HandleInput("k") // Select BottomRight of current
	assert.True(t, completed3)
	assert.Equal(t, 2, manager.CurrentDepth(), "Should still be at max depth")
	assert.NotEqual(t, point2, point3, "Should return different point (sub-cell center)")
}

func TestHandleInput_InvalidKeyLength_FallsBackToDefault(t *testing.T) {
	keys := "€ab" // 3 runes, invalid length
	logger := zap.NewNop()
	screenBounds := image.Rect(0, 0, 100, 100)
	manager := recursivegrid.NewManager(
		screenBounds,
		keys,
		",",
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

func TestHandleInput_ValidMultibyteKeys(t *testing.T) {
	keys := "€abc" // 4 runes, valid length
	logger := zap.NewNop()
	screenBounds := image.Rect(0, 0, 100, 100)
	manager := recursivegrid.NewManager(
		screenBounds,
		keys,
		",",
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
