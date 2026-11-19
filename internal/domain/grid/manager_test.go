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

	var opened image.Rectangle

	gridManager := grid.NewManager(
		testGrid,
		domain.GridDimensions{Rows: 3, Cols: 3}, "asdf",
		func(bool) {},
		func(cell *grid.Cell) { opened = cell.Bounds() },
		logger,
	)

	gridRouter := grid.NewRouter(gridManager, logger)

	t.Run("Grid routing workflow", func(t *testing.T) {
		gridRouter.RouteKey("a")

		result2 := gridRouter.RouteKey("s")

		if result2.Complete() {
			t.Error("Expected not complete on two characters")
		}

		result3 := gridRouter.RouteKey("d")

		if result3.Complete() {
			t.Error("Expected not complete on three characters")
		}

		if want := testGrid.CellByCoordinate("ASD"); want == nil || opened != want.Bounds() {
			t.Fatalf("subgrid opened on %v, want the cell labeled ASD", opened)
		}

		result4 := gridRouter.RouteKey("f")

		if !result4.Complete() {
			t.Fatal("Expected complete on fourth character")
		}

		// "f" is the fourth subgrid key, so it names the fourth subgrid cell.
		cell := grid.SubgridCells(opened, domain.GridDimensions{Rows: 3, Cols: 3})[3]
		if target := result4.TargetPoint(); !target.In(cell) {
			t.Errorf("target %v is outside the subgrid cell %v that f names", target, cell)
		}
	})

	t.Run("Grid escape and tab handling", func(t *testing.T) {
		result := gridRouter.RouteKey("escape")

		if result.Complete() {
			t.Error("Expected not complete on escape")
		}

		// Tab is not one of the grid's coordinate characters and no subgrid is
		// active here, so it must be rejected outright. It must not complete,
		// and it must not touch the coordinate input the user has typed so far.
		// An unmapped key that silently landed in the input buffer would
		// desynchronise the overlay from the manager.
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

	// "AB" is made of valid characters and continues a real coordinate, so
	// only the single-character guard refuses it.
	point, complete := manager.HandleInput("AB")
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

func TestManager_PrefixValidationRegression(t *testing.T) {
	// Typing an invalid sequence used to leave the grid empty.
	logger := logger.Get()

	// A grid labeled from the two characters A and B.
	testGrid := grid.NewGrid("AB", image.Rect(0, 0, 100, 100), logger)
	manager := grid.NewManager(
		testGrid,
		domain.GridDimensions{Rows: 2, Cols: 2},
		"ab",
		nil,
		nil,
		logger,
	)

	cells := testGrid.AllCells()

	coordinates := make([]string, len(cells))
	for i, cell := range cells {
		coordinates[i] = cell.Coordinate()
	}

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

	// Invalid characters are rejected.
	manager.Reset()

	_, complete := manager.HandleInput("Z") // Z is not in valid character set "AB"
	if complete {
		t.Error("Expected not complete for invalid character Z")
	}

	if input := manager.CurrentInput(); input != "" {
		t.Errorf("CurrentInput() = %q, want '' for invalid character", input)
	}

	// A prefix one character shorter than a real coordinate is accepted.
	prefixLength := len(testCoord) - 1
	if prefixLength > 0 {
		validPrefix := testCoord[:prefixLength]

		manager.Reset()

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

func TestManager_HandleInput_SubgridDisabled(t *testing.T) {
	log := logger.Get()
	testGrid := grid.NewGrid("abcdefghijklmnopqrstuvwxyz", image.Rect(0, 0, 1000, 1000), log)
	cell := testGrid.Cells()[len(testGrid.Cells())/2]
	label := cell.Coordinate()

	subgridOpens := 0
	manager := grid.NewManager(
		testGrid,
		domain.GridDimensions{Rows: 3, Cols: 3}, "asdfghjkl",
		func(bool) {},
		func(*grid.Cell) { subgridOpens++ },
		log,
	)
	manager.SetSubgridEnabled(false)

	var (
		point    image.Point
		complete bool
	)

	for i, key := range strings.Split(label, "") {
		point, complete = manager.HandleInput(key)
		if last := i == len(label)-1; complete != last {
			t.Fatalf("key %d of %q: complete = %v, want %v", i+1, label, complete, last)
		}
	}

	if point != cell.Center() {
		t.Errorf("point = %v, want the cell center %v", point, cell.Center())
	}

	if subgridOpens != 0 {
		t.Errorf("subgrid opened %d times, want 0", subgridOpens)
	}

	if got, picked := manager.SelectedCell(); !picked || got != cell {
		t.Errorf("SelectedCell = %v, %v, want the labeled cell", got, picked)
	}

	if manager.CurrentInput() != label {
		t.Errorf("input = %q, want the label %q kept for backspace", manager.CurrentInput(), label)
	}

	if _, moved := manager.MoveDirection(domain.DirectionRight, 1); moved {
		t.Error("MoveDirection moved a cell with no subgrid open")
	}

	manager.HandleBackspace()

	if manager.CurrentInput() != label[:len(label)-1] {
		t.Errorf(
			"input after backspace = %q, want %q",
			manager.CurrentInput(),
			label[:len(label)-1],
		)
	}

	if _, picked := manager.SelectedCell(); picked {
		t.Error("SelectedCell still reports a cell after backspace shortened the label")
	}
}
