package grid_test

import (
	"image"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/grid"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
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
	// Use unique parameters to avoid cache conflicts
	testGrid := grid.NewGrid("ABCD", image.Rect(0, 0, 50, 50), logger)

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

func TestManager_AcceptsNonLetterCharacters(t *testing.T) {
	logger := logger.Get()
	// Create grid with only numbers and symbols
	testGrid := grid.NewGrid("123!@", image.Rect(0, 0, 500, 500), logger)

	manager := grid.NewManager(testGrid, 2, 2, "ab", nil, nil, logger)

	// Test that numbers are accepted
	_, complete := manager.HandleInput("1")
	if complete {
		t.Error("Expected not complete after single number")
	}

	if input := manager.CurrentInput(); input != "1" {
		t.Errorf("CurrentInput() = %q, want '1'", input)
	}

	manager.Reset()

	// Test that symbols are accepted
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

	// Check that ValidCharacters includes the comma
	validChars := testGrid.ValidCharacters()
	if !strings.Contains(validChars, ",") {
		t.Errorf("ValidCharacters() = %q, should contain ','", validChars)
	}

	manager := grid.NewManager(testGrid, 2, 2, "ab", nil, nil, logger)

	// Test that regular characters work
	_, complete := manager.HandleInput("A")
	if complete {
		t.Error("Expected not complete after A")
	}

	if input := manager.CurrentInput(); input != "A" {
		t.Errorf("CurrentInput() = %q, want 'A'", input)
	}

	// Test that symbols from row_labels are accepted only if they lead to valid coordinates
	// Note: "," is now the reset key, so we use "." instead for testing symbols
	_, complete = manager.HandleInput(".")
	if complete {
		t.Error("Expected not complete after period")
	}

	// Since "A." doesn't match any coordinate prefix, it should be rejected
	if input := manager.CurrentInput(); input != "A" {
		t.Errorf("CurrentInput() = %q, want 'A' (period should be rejected)", input)
	}

	// Test that a valid next character is accepted
	_, complete = manager.HandleInput("A") // "AA" should match some coordinates
	if complete {
		t.Error("Expected not complete after AA")
	}

	if input := manager.CurrentInput(); input != "AA" {
		t.Errorf("CurrentInput() = %q, want 'AA'", input)
	}

	// Test that reset key clears the input
	_, complete = manager.HandleInput(",") // "," is the reset key
	if complete {
		t.Error("Expected not complete after reset")
	}

	if input := manager.CurrentInput(); input != "" {
		t.Errorf("CurrentInput() = %q, want '' (reset should clear input)", input)
	}

	// Test that invalid character is rejected (input stays empty after reset)
	_, complete = manager.HandleInput("Z") // Z not in valid characters
	if complete {
		t.Error("Expected not complete for invalid character")
	}

	// Input should still be empty after reset + invalid char
	if input := manager.CurrentInput(); input != "" {
		t.Errorf("CurrentInput() = %q, want '' (input stays empty after reset)", input)
	}

	// Now test that a valid character after reset is accepted
	_, complete = manager.HandleInput("A")
	if complete {
		t.Error("Expected not complete after A following reset")
	}

	if input := manager.CurrentInput(); input != "A" {
		t.Errorf("CurrentInput() = %q, want 'A'", input)
	}
}

func TestManager_InputValidation(t *testing.T) {
	logger := logger.Get()

	// Create a simple grid with known coordinates: AA, AB, AC, BA, BB, BC, CA, CB, CC
	testGrid := grid.NewGrid("ABC", image.Rect(0, 0, 100, 100), logger)
	manager := grid.NewManager(testGrid, 2, 2, "ab", nil, nil, logger)

	// Test 1: Valid first character should be accepted
	_, complete := manager.HandleInput("A")
	if complete {
		t.Error("Expected not complete after first character")
	}

	if input := manager.CurrentInput(); input != "A" {
		t.Errorf("CurrentInput() = %q, want 'A'", input)
	}

	// Test 2: Valid second character that completes coordinate should enter subgrid
	_, complete = manager.HandleInput("A") // "AA" is a valid coordinate - enters subgrid
	if complete {
		t.Error("Expected not complete after 'AA' (enters subgrid)")
	}
	// When entering subgrid, input is reset to ""
	if input := manager.CurrentInput(); input != "" {
		t.Errorf("CurrentInput() = %q, want '' (reset for subgrid)", input)
	}

	// Test 3: Invalid character that doesn't lead to matches should be rejected
	manager.Reset() // Start fresh

	// Put in a state where next character would be invalid
	_, _ = manager.HandleInput("A")

	// Try a character that doesn't continue any valid coordinate
	invalidChar := "Z" // Z doesn't appear in any coordinate starting with A

	_, complete = manager.HandleInput(invalidChar)
	if complete {
		t.Error("Expected not complete for invalid continuation")
	}
	// Input should remain "A" since invalid char was rejected
	if input := manager.CurrentInput(); input != "A" {
		t.Errorf("CurrentInput() = %q, want 'A' (invalid char should be rejected)", input)
	}

	// Test 4: Backspace should still work
	_, complete = manager.HandleInput("\x7f") // backspace
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
	manager := grid.NewManager(testGrid, 2, 2, "ab", nil, nil, logger)

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
