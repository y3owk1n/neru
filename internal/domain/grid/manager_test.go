package grid_test

import (
	"image"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/logger"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/grid"
)

func TestGridManager_RouterIntegration(t *testing.T) {
	logger := logger.Get()

	testGrid := grid.NewGrid("abcdefghijklmnopqrstuvwxyz", image.Rect(0, 0, 1000, 1000), logger)

	gridManager := grid.NewManager(
		testGrid,
		domain.GridDimensions{Rows: 3, Cols: 3}, "asdf",
		func(redraw bool) {
		},
		func(cell *grid.Cell) {
		},
		logger,
	)

	gridRouter := grid.NewRouter(gridManager, logger)

	t.Run("Grid routing workflow", func(t *testing.T) {
		gridRouter.RouteKey("a")

		result2 := gridRouter.RouteKey("s")

		if result2.Complete() {
			t.Error("Expected not complete on two characters")
		}

		// Test typing "d" - still not complete
		result3 := gridRouter.RouteKey("d")

		if result3.Complete() {
			t.Error("Expected not complete on three characters")
		}

		result4 := gridRouter.RouteKey("f")

		if !result4.Complete() {
			t.Error("Expected complete on fourth character")
		}

		targetPoint := result4.TargetPoint()
		if targetPoint.X == 0 || targetPoint.Y == 0 {
			t.Error("Expected non-zero target point")
		}
	})

	t.Run("Grid manager input handling", func(t *testing.T) {
		gridManager.HandleInput("a")

		pointTab, completeTab := gridManager.HandleInput("\t")
		if completeTab {
			t.Logf("Tab completed selection at point: %v", pointTab)
		}

		gridManager.HandleInput("s")
		gridManager.HandleInput("d")
		point4, _ := gridManager.HandleInput("f")

		if point4.X == 0 && point4.Y == 0 {
			t.Error("Expected non-zero point on coordinate input")
		}
	})

	t.Run("Grid escape and tab handling", func(t *testing.T) {
		result := gridRouter.RouteKey("escape")

		if result.Complete() {
			t.Error("Expected not complete on escape")
		}

		// Tab is not one of the grid's coordinate characters and no subgrid is
		// active here, so it must be rejected outright: no completion, and —
		// just as important — no effect on the coordinate input the user has
		// typed so far. An unmapped key that silently landed in the input
		// buffer would desynchronise the overlay from the manager.
		beforeInput := gridManager.CurrentInput()

		resultTab := gridRouter.RouteKey("\t")

		if resultTab.Complete() {
			t.Errorf(
				"Expected tab not to complete a selection, got target %v",
				resultTab.TargetPoint(),
			)
		}

		if got := gridManager.CurrentInput(); got != beforeInput {
			t.Errorf("tab changed the coordinate input from %q to %q", beforeInput, got)
		}
	})
}

func TestManager_CurrentInput(t *testing.T) {
	logger := logger.Get()
	testGrid := grid.NewGrid("ABCD", image.Rect(0, 0, 100, 100), logger)

	manager := grid.NewManager(
		testGrid,
		domain.GridDimensions{Rows: 2, Cols: 2},
		"12",
		nil,
		nil,
		logger,
	)

	if input := manager.CurrentInput(); input != "" {
		t.Errorf("CurrentInput() = %q, want empty", input)
	}

	manager.HandleInput("A")

	if input := manager.CurrentInput(); input != "A" {
		t.Errorf("CurrentInput() = %q, want 'A'", input)
	}
}

func TestManager_Reset(t *testing.T) {
	logger := logger.Get()
	testGrid := grid.NewGrid("ABCD", image.Rect(0, 0, 50, 50), logger)

	manager := grid.NewManager(
		testGrid,
		domain.GridDimensions{Rows: 2, Cols: 2},
		"12",
		nil,
		nil,
		logger,
	)

	manager.HandleInput("A")

	if input := manager.CurrentInput(); input != "A" {
		t.Errorf("Before reset, CurrentInput() = %q", input)
	}

	manager.Reset()

	if input := manager.CurrentInput(); input != "" {
		t.Errorf("After reset, CurrentInput() = %q, want empty", input)
	}
}

func TestManager_IgnoresNonSingleCharKey(t *testing.T) {
	logger := logger.Get()
	testGrid := grid.NewGrid("ABC", image.Rect(0, 0, 300, 300), logger)

	manager := grid.NewManager(
		testGrid,
		domain.GridDimensions{Rows: 2, Cols: 2},
		"12",
		nil,
		nil,
		logger,
	)

	manager.HandleInput("A")

	if manager.CurrentInput() != "A" {
		t.Fatalf("expected input 'A' before reset, got %q", manager.CurrentInput())
	}

	point, complete := manager.HandleInput("Ctrl+R")
	if complete {
		t.Fatalf("multi-character key should not complete selection")
	}

	if point.X != 0 || point.Y != 0 {
		t.Fatalf("multi-character key should not return a point, got %v", point)
	}

	if manager.CurrentInput() != "A" {
		t.Fatalf("expected input to stay unchanged, got %q", manager.CurrentInput())
	}
}

func TestManager_AcceptsNonLetterCharacters(t *testing.T) {
	logger := logger.Get()
	testGrid := grid.NewGrid("123!@", image.Rect(0, 0, 500, 500), logger)

	manager := grid.NewManager(
		testGrid,
		domain.GridDimensions{Rows: 2, Cols: 2},
		"ab",
		nil,
		nil,
		logger,
	)

	_, complete := manager.HandleInput("1")
	if complete {
		t.Error("Expected not complete after single number")
	}

	if input := manager.CurrentInput(); input != "1" {
		t.Errorf("CurrentInput() = %q, want '1'", input)
	}

	manager.Reset()

	_, complete2 := manager.HandleInput("!")
	if complete2 {
		t.Error("Expected not complete after single symbol")
	}

	if input := manager.CurrentInput(); input != "!" {
		t.Errorf("CurrentInput() = %q, want '!'", input)
	}
}

func TestManager_CustomLabelsWithSymbols(t *testing.T) {
	logger := logger.Get()

	// Create grid with custom labels containing symbols
	testGrid := grid.NewGridWithLabels("ABC", "',.PYF", "AOEU", image.Rect(0, 0, 300, 300), logger)

	validChars := testGrid.ValidCharacters()
	if !strings.Contains(validChars, ",") {
		t.Errorf("ValidCharacters() = %q, should contain ','", validChars)
	}

	manager := grid.NewManager(
		testGrid,
		domain.GridDimensions{Rows: 2, Cols: 2},
		"ab",
		nil,
		nil,
		logger,
	)

	_, complete := manager.HandleInput("A")
	if complete {
		t.Error("Expected not complete after A")
	}

	if input := manager.CurrentInput(); input != "A" {
		t.Errorf("CurrentInput() = %q, want 'A'", input)
	}

	_, complete = manager.HandleInput(".")
	if complete {
		t.Error("Expected not complete after period")
	}

	if input := manager.CurrentInput(); input != "A" {
		t.Errorf("CurrentInput() = %q, want 'A' (period should be rejected)", input)
	}

	_, complete = manager.HandleInput("A") // "AA" should match some coordinates
	if complete {
		t.Error("Expected not complete after AA")
	}

	if input := manager.CurrentInput(); input != "AA" {
		t.Errorf("CurrentInput() = %q, want 'AA'", input)
	}

	manager.Reset()

	if input := manager.CurrentInput(); input != "" {
		t.Errorf("CurrentInput() = %q, want '' after reset", input)
	}

	_, complete = manager.HandleInput("Z") // Z not in valid characters
	if complete {
		t.Error("Expected not complete for invalid character")
	}

	if input := manager.CurrentInput(); input != "" {
		t.Errorf("CurrentInput() = %q, want '' (input stays empty after reset)", input)
	}

	_, complete = manager.HandleInput("A")
	if complete {
		t.Error("Expected not complete after A following reset")
	}

	if input := manager.CurrentInput(); input != "A" {
		t.Errorf("CurrentInput() = %q, want 'A'", input)
	}
}

func TestGridManager_ResetSilentClearsInputWithoutCallback(t *testing.T) {
	logger := logger.Get()

	testGrid := grid.NewGrid("abcdefghijklmnopqrstuvwxyz", image.Rect(0, 0, 1000, 1000), logger)

	var updates int

	manager := grid.NewManager(
		testGrid,
		domain.GridDimensions{Rows: 3, Cols: 3}, "asdf",
		func(_ bool) { updates++ },
		func(_ *grid.Cell) {},
		logger,
	)

	manager.SetCurrentInput("AB")

	manager.ResetSilent()

	if input := manager.CurrentInput(); input != "" {
		t.Errorf("CurrentInput() = %q, want '' after ResetSilent()", input)
	}

	if updates != 0 {
		t.Errorf("ResetSilent() fired onUpdate %d times, want 0", updates)
	}
}

func TestGridManager_ResetClearsInputAndFiresCallback(t *testing.T) {
	logger := logger.Get()

	testGrid := grid.NewGrid("abcdefghijklmnopqrstuvwxyz", image.Rect(0, 0, 1000, 1000), logger)

	var redraws []bool

	manager := grid.NewManager(
		testGrid,
		domain.GridDimensions{Rows: 3, Cols: 3}, "asdf",
		func(redraw bool) { redraws = append(redraws, redraw) },
		func(_ *grid.Cell) {},
		logger,
	)

	manager.SetCurrentInput("AB")

	manager.Reset()

	if input := manager.CurrentInput(); input != "" {
		t.Errorf("CurrentInput() = %q, want '' after Reset()", input)
	}

	if len(redraws) != 1 || redraws[0] {
		t.Errorf("Reset() onUpdate calls = %v, want exactly one call with redraw=false", redraws)
	}
}

func TestManager_InputValidation(t *testing.T) {
	logger := logger.Get()

	testGrid := grid.NewGrid("ABC", image.Rect(0, 0, 100, 100), logger)
	manager := grid.NewManager(
		testGrid,
		domain.GridDimensions{Rows: 2, Cols: 2},
		"ab",
		nil,
		nil,
		logger,
	)

	_, complete := manager.HandleInput("A")
	if complete {
		t.Error("Expected not complete after first character")
	}

	if input := manager.CurrentInput(); input != "A" {
		t.Errorf("CurrentInput() = %q, want 'A'", input)
	}

	_, complete = manager.HandleInput("A") // "AA" is a valid coordinate - enters subgrid
	if complete {
		t.Error("Expected not complete after 'AA' (enters subgrid)")
	}

	if input := manager.CurrentInput(); input != "" {
		t.Errorf("CurrentInput() = %q, want '' (reset for subgrid)", input)
	}

	manager.Reset() // Start fresh

	_, _ = manager.HandleInput("A")

	invalidChar := "Z" // Z doesn't appear in any coordinate starting with A

	_, complete = manager.HandleInput(invalidChar)
	if complete {
		t.Error("Expected not complete for invalid continuation")
	}
	// Input should remain "A" since invalid char was rejected
	if input := manager.CurrentInput(); input != "A" {
		t.Errorf("CurrentInput() = %q, want 'A' (invalid char should be rejected)", input)
	}

	// Test 4: Backspace should still work through explicit manager API
	_, complete = manager.HandleBackspace()
	if complete {
		t.Error("Expected not complete after backspace")
	}

	if input := manager.CurrentInput(); input != "" {
		t.Errorf("CurrentInput() = %q, want '' after backspace", input)
	}

	// Test 5: Completely invalid character (not in valid characters) should be rejected
	_, complete = manager.HandleInput("9") // 9 is not in valid characters "ABC"
	if complete {
		t.Error("Expected not complete for invalid character")
	}

	if input := manager.CurrentInput(); input != "" {
		t.Errorf("CurrentInput() = %q, want '' (invalid char should be rejected)", input)
	}

	// Test 6: Valid partial input should be accepted
	manager.Reset()

	_, complete = manager.HandleInput("B")
	if complete {
		t.Error("Expected not complete after 'B'")
	}

	if input := manager.CurrentInput(); input != "B" {
		t.Errorf("CurrentInput() = %q, want 'B'", input)
	}

	_, complete = manager.HandleInput("C") // "BC" is valid - enters subgrid
	if complete {
		t.Error("Expected not complete after 'BC' (enters subgrid)")
	}
	// When entering subgrid, input is reset to ""
	if input := manager.CurrentInput(); input != "" {
		t.Errorf("CurrentInput() = %q, want '' (reset for subgrid)", input)
	}
}

func TestManager_PrefixValidationRegression(t *testing.T) {
	// Regression test specifically for the issue where typing invalid sequences
	// would cause the grid to become empty
	logger := logger.Get()

	// Create a grid with known coordinates: AA, AB, BA, BB (for "AB" characters)
	testGrid := grid.NewGrid("AB", image.Rect(0, 0, 100, 100), logger)
	manager := grid.NewManager(
		testGrid,
		domain.GridDimensions{Rows: 2, Cols: 2},
		"ab",
		nil,
		nil,
		logger,
	)

	// Get all coordinates
	cells := testGrid.AllCells()

	coordinates := make([]string, len(cells))
	for i, cell := range cells {
		coordinates[i] = cell.Coordinate()
	}

	// Test that we can build up valid coordinates
	testCoord := coordinates[0] // Test with first coordinate "AAAA"

	manager.Reset()

	for position, char := range testCoord {
		_, complete := manager.HandleInput(string(char))
		if complete {
			t.Errorf(
				"Coordinate %q should not complete at position %d (enters subgrid at end)",
				testCoord,
				position,
			)
		}

		if position < len(testCoord)-1 {
			// Before the last character, input accumulates
			expectedInput := testCoord[:position+1]
			if input := manager.CurrentInput(); input != expectedInput {
				t.Errorf(
					"For coordinate %q at position %d: CurrentInput() = %q, want %q",
					testCoord,
					position,
					input,
					expectedInput,
				)
			}
		} else {
			// After the last character, enters subgrid and resets input
			if input := manager.CurrentInput(); input != "" {
				t.Errorf(
					"For coordinate %q at position %d: CurrentInput() = %q, want '' (reset for subgrid)",
					testCoord,
					position,
					input,
				)
			}
		}
	}

	// Test that invalid characters are rejected
	manager.Reset()

	_, complete := manager.HandleInput("Z") // Z is not in valid character set "AB"
	if complete {
		t.Error("Expected not complete for invalid character Z")
	}

	if input := manager.CurrentInput(); input != "" {
		t.Errorf("CurrentInput() = %q, want '' for invalid character", input)
	}

	// Test that valid partial prefix works - derive from actual coordinate
	// Use a prefix that's shorter than the complete coordinate
	prefixLength := len(testCoord) - 1 // One character shorter than complete
	if prefixLength > 0 {
		validPrefix := testCoord[:prefixLength]

		manager.Reset()

		// Type each character of the prefix
		for index, char := range validPrefix {
			_, complete := manager.HandleInput(string(char))
			if complete {
				t.Errorf("Prefix %q should not complete at position %d", validPrefix, index)
			}

			expectedInput := validPrefix[:index+1]
			if input := manager.CurrentInput(); input != expectedInput {
				t.Errorf(
					"After typing %q: CurrentInput() = %q, want %q",
					validPrefix[:index+1],
					input,
					expectedInput,
				)
			}
		}
	}
}
