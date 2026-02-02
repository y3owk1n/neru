package quadgrid_test

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/y3owk1n/neru/internal/core/domain/quadgrid"
	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()

	manager := quadgrid.NewManager(
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

	manager := quadgrid.NewManager(
		bounds,
		"", // Empty keys - should use default
		",",
		[]string{"escape"},
		nil,
		nil,
		logger,
	)

	assert.Equal(t, quadgrid.DefaultKeys, manager.Keys(), "Should use default keys when empty")
}

func TestManagerHandleInputQuadrantSelection(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()

	updateCalled := false
	manager := quadgrid.NewManager(
		bounds,
		"uijk",
		",",
		[]string{"escape"},
		func() { updateCalled = true },
		nil,
		logger,
	)

	// Select top-left quadrant (key 'u')
	point, completed, shouldExit := manager.HandleInput("u")

	assert.Equal(t, image.Point{X: 25, Y: 25}, point, "Should return center of top-left quadrant")
	assert.False(t, completed, "Should not be completed")
	assert.False(t, shouldExit, "Should not exit")
	assert.True(t, updateCalled, "Update callback should be called")
	assert.Equal(t, 1, manager.CurrentDepth(), "Depth should be 1")
}

func TestManagerHandleInputExitKey(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()

	manager := quadgrid.NewManager(
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
	manager := quadgrid.NewManager(
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

	assert.Equal(t, image.Point{}, point, "Should return zero point")
	assert.False(t, completed, "Should not be completed")
	assert.False(t, shouldExit, "Should not exit")
	assert.True(t, updateCalled, "Update callback should be called")
	assert.Equal(t, 0, manager.CurrentDepth(), "Depth should be reset to 0")
}

func TestManagerHandleInputBacktrack(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()

	updateCalled := false
	manager := quadgrid.NewManager(
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

	assert.Equal(t, image.Point{}, point, "Should return zero point")
	assert.False(t, completed, "Should not be completed")
	assert.False(t, shouldExit, "Should not exit")
	assert.True(t, updateCalled, "Update callback should be called")
	assert.Equal(t, 0, manager.CurrentDepth(), "Depth should be 0 after backtrack")
}

func TestManagerHandleInputUnmappedKey(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	logger := zap.NewNop()

	updateCalled := false
	manager := quadgrid.NewManager(
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

	manager := quadgrid.NewManagerWithConfig(
		bounds,
		"uijk",
		",",
		[]string{"escape"},
		50, // minSize large enough to complete quickly
		10,
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
	manager := quadgrid.NewManagerWithConfig(
		bounds,
		"uijk",
		",",
		[]string{"escape"},
		1, // minSize
		2, // maxDepth
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
	// Should return sub-quadrant center but NOT change depth
	point3, completed3, _ := manager.HandleInput("k") // Select BottomRight of current
	assert.True(t, completed3)
	assert.Equal(t, 2, manager.CurrentDepth(), "Should still be at max depth")
	assert.NotEqual(t, point2, point3, "Should return different point (sub-quadrant center)")
}

func TestHandleInput_MultibyteKeys(t *testing.T) {
	keys := "€abc"
	logger := zap.NewNop()
	screenBounds := image.Rect(0, 0, 100, 100)
	m := quadgrid.NewManager(screenBounds, keys, ",", []string{"escape"}, nil, nil, logger)
	assert.Equal(t, keys, m.Keys(), "Should accept multibyte keys")
	assert.NotPanics(t, func() {
		_, _, shouldExit := m.HandleInput("c")
		assert.False(t, shouldExit, "Should not exit")
	}, "HandleInput should not panic with multibyte keys")
}
