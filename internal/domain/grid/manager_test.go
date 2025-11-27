package grid_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/infra/logger"
)

func TestGridManager_RouterIntegration(t *testing.T) {
	logger := logger.Get()

	// Create a test grid
	testGrid := grid.NewGrid("abcdefghijklmnopqrstuvwxyz", image.Rect(0, 0, 1000, 1000), logger)

	// Create grid manager
	gridManager := grid.NewManager(
		testGrid,
		3, 3, "asdf",
		func(redraw bool) {
			// Update callback
		},
		func(cell *grid.Cell) {
			// Show sub callback
		},
		logger,
	)

	// Create grid router
	gridRouter := grid.NewRouter(gridManager, logger)

	t.Run("Grid routing workflow", func(t *testing.T) {
		// Test typing "a" - should be valid input (4-char labels needed)
		result1 := gridRouter.RouteKey("a")
		if result1.Exit() {
			t.Error("Expected not to exit on 'a'")
		}

		// Test typing "s" - still not complete
		result2 := gridRouter.RouteKey("s")
		if result2.Exit() {
			t.Error("Expected not to exit on 's'")
		}

		if result2.Complete() {
			t.Error("Expected not complete on two characters")
		}

		// Test typing "d" - still not complete
		result3 := gridRouter.RouteKey("d")
		if result3.Exit() {
			t.Error("Expected not to exit on 'd'")
		}

		if result3.Complete() {
			t.Error("Expected not complete on three characters")
		}

		// Test typing "f" - should complete coordinate
		result4 := gridRouter.RouteKey("f")
		if result4.Exit() {
			t.Error("Expected not to exit on 'f'")
		}

		if !result4.Complete() {
			t.Error("Expected complete on fourth character")
		}

		// Check target point
		targetPoint := result4.TargetPoint()
		if targetPoint.X == 0 || targetPoint.Y == 0 {
			t.Error("Expected non-zero target point")
		}
	})

	t.Run("Grid manager input handling", func(t *testing.T) {
		// Test input handling - this tests the grid manager's coordinate processing
		// With subgrid enabled, single characters might complete selections
		gridManager.HandleInput("a")

		// Test tab key handling - should move to next subgrid
		pointTab, completeTab := gridManager.HandleInput("\t") // Tab key
		if completeTab {
			t.Logf("Tab completed selection at point: %v", pointTab)
		}

		// Test that we can handle multiple inputs
		gridManager.HandleInput("s")
		gridManager.HandleInput("d")
		point4, _ := gridManager.HandleInput("f")

		if point4.X == 0 && point4.Y == 0 {
			t.Error("Expected non-zero point on coordinate input")
		}
	})

	t.Run("Grid escape and tab handling", func(t *testing.T) {
		// Test escape key
		result := gridRouter.RouteKey("escape")
		if !result.Exit() {
			t.Error("Expected to exit on escape")
		}

		if result.Complete() {
			t.Error("Expected not complete on escape")
		}

		// Test tab key - should handle subgrid navigation
		resultTab := gridRouter.RouteKey("\t")
		// Tab behavior depends on subgrid state, just ensure it doesn't crash
		_ = resultTab
	})
}

func TestManager_CurrentInput(t *testing.T) {
	logger := logger.Get()
	testGrid := grid.NewGrid("ABCD", image.Rect(0, 0, 100, 100), logger)

	manager := grid.NewManager(testGrid, 2, 2, "12", nil, nil, logger)

	// Initially empty
	if input := manager.CurrentInput(); input != "" {
		t.Errorf("CurrentInput() = %q, want empty", input)
	}

	// After input
	manager.HandleInput("A")

	if input := manager.CurrentInput(); input != "A" {
		t.Errorf("CurrentInput() = %q, want 'A'", input)
	}
}

func TestManager_Reset(t *testing.T) {
	logger := logger.Get()
	testGrid := grid.NewGrid("ABCD", image.Rect(0, 0, 100, 100), logger)

	manager := grid.NewManager(testGrid, 2, 2, "12", nil, nil, logger)

	manager.HandleInput("A")

	if input := manager.CurrentInput(); input != "A" {
		t.Errorf("Before reset, CurrentInput() = %q", input)
	}

	manager.Reset()

	if input := manager.CurrentInput(); input != "" {
		t.Errorf("After reset, CurrentInput() = %q, want empty", input)
	}
}
