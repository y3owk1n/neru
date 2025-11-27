//go:build integration
// +build integration

package domain_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/infra/logger"
)

// TestHintManagerIntegration tests the hint manager and router working together
func TestHintManagerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logger.Get()

	// Create hint manager
	hintManager := hint.NewManager(logger)

	// Create hint router
	hintRouter := hint.NewRouter(hintManager, logger)

	// Create some test elements
	elem1, _ := element.NewElement("elem1", image.Rect(10, 10, 50, 50), element.RoleButton)
	elem2, _ := element.NewElement("elem2", image.Rect(60, 10, 100, 50), element.RoleButton)
	elem3, _ := element.NewElement("elem3", image.Rect(10, 60, 50, 100), element.RoleButton)
	testElements := []*element.Element{elem1, elem2, elem3}

	// Create hint generator
	gen, err := hint.NewAlphabetGenerator("asdf")
	if err != nil {
		t.Fatalf("Failed to create hint generator: %v", err)
	}

	// Generate hints
	hintInterfaces, err := gen.Generate(context.Background(), testElements)
	if err != nil {
		t.Fatalf("Failed to generate hints: %v", err)
	}

	// Create hint collection
	hints := hint.NewCollection(hintInterfaces)

	// Set hints in manager
	hintManager.SetHints(hints)

	t.Run("Hint manager and router integration", func(t *testing.T) {
		// Test that manager and router can work together
		// Test escape - should exit
		result := hintRouter.RouteKey("escape")
		if !result.Exit() {
			t.Error("Expected to exit on escape")
		}
	})

	t.Run("Hint manager callback integration", func(t *testing.T) {
		var callbackCalled bool

		// Set callback
		hintManager.SetUpdateCallback(func(hints []*hint.Interface) {
			callbackCalled = true
		})

		// Reset should trigger callback
		hintManager.Reset()
		if !callbackCalled {
			t.Error("Expected callback to be called on reset")
		}
	})
}

// TestGridManagerIntegration tests the grid manager and router working together
func TestGridManagerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

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
		if result1.Complete() {
			t.Error("Expected not complete on single character")
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

// TestDomainLayerIntegration tests interactions between different domain components
func TestDomainLayerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logger.Get()

	// Test hint and grid generators working together
	t.Run("Hint and grid generators", func(t *testing.T) {
		// Create hint generator
		hintGen, err := hint.NewAlphabetGenerator("asdfqwert")
		if err != nil {
			t.Fatalf("Failed to create hint generator: %v", err)
		}

		// Create grid
		grid := grid.NewGrid("asdf", image.Rect(0, 0, 800, 600), logger)

		// Both should work independently
		testElem, _ := element.NewElement("test1", image.Rect(10, 10, 100, 100), element.RoleButton)
		testElements := []*element.Element{testElem}

		hintInterfaces, err := hintGen.Generate(context.Background(), testElements)
		if err != nil {
			t.Fatalf("Failed to generate hints: %v", err)
		}
		if len(hintInterfaces) != 1 {
			t.Errorf("Expected 1 hint, got %d", len(hintInterfaces))
		}
		if len(hintInterfaces) != 1 {
			t.Errorf("Expected 1 hint, got %d", len(hintInterfaces))
		}

		if grid == nil {
			t.Error("Expected grid to be created")
		}

		// Test they can coexist
		cells := grid.Cells()
		if len(cells) == 0 {
			t.Error("Expected grid to have cells")
		}
	})
}
