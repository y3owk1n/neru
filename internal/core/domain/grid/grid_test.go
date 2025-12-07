package grid_test

import (
	"image"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/grid"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
)

const testCharacters = "ABC"

func TestGrid_Initialization(t *testing.T) {
	log := logger.Get()
	tests := []struct {
		name      string
		chars     string
		bounds    image.Rectangle
		wantCells int
	}{
		{
			name:      "standard 1080p",
			chars:     "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
			bounds:    image.Rect(0, 0, 1920, 1080),
			wantCells: 26 * 26, // 2 chars depth
		},
		{
			name:      "small screen",
			chars:     "ABC",
			bounds:    image.Rect(0, 0, 100, 100),
			wantCells: 3 * 3, // 2 chars depth
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			grid := grid.NewGrid(testCase.chars, testCase.bounds, log)
			if len(grid.AllCells()) == 0 {
				t.Error("Expected cells to be generated")
			}
		})
	}
}

func TestGrid_CellByCoordinate(t *testing.T) {
	logger := logger.Get()
	grid := grid.NewGrid("ABC", image.Rect(0, 0, 300, 300), logger)

	// Get a valid coordinate from the generated grid
	cells := grid.AllCells()
	if len(cells) == 0 {
		t.Fatal("Expected cells to be generated")
	}

	validCoordinate := cells[0].Coordinate()

	tests := []struct {
		name  string
		coord string
		want  bool // exists
	}{
		{"valid " + validCoordinate, validCoordinate, true},
		{"invalid ZZZ", "ZZZ", false},
		{"lowercase coordinate", strings.ToLower(validCoordinate), true},
		{"empty coordinate", "", false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cell := grid.CellByCoordinate(testCase.coord)
			if (cell != nil) != testCase.want {
				t.Errorf(
					"CellByCoordinate(%q) exists = %v, want %v",
					testCase.coord,
					cell != nil,
					testCase.want,
				)
			}
		})
	}
}

func TestCell_Methods(t *testing.T) {
	logger := logger.Get()
	grid := grid.NewGrid("ABC", image.Rect(0, 0, 300, 300), logger)

	cells := grid.AllCells()
	if len(cells) == 0 {
		t.Fatal("Expected cells to be generated")
	}

	cell := cells[0]

	// Test that methods return non-zero values
	if cell.Coordinate() == "" {
		t.Error("Coordinate() returned empty string")
	}

	bounds := cell.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		t.Errorf("Bounds() returned invalid bounds: %v", bounds)
	}

	center := cell.Center()
	if center.X < 0 || center.Y < 0 {
		t.Errorf("Center() returned invalid center: %v", center)
	}

	// Test that center is within bounds
	if !center.In(bounds) {
		t.Errorf("Center %v is not within bounds %v", center, bounds)
	}
}

func TestGrid_Getters(t *testing.T) {
	logger := logger.Get()
	bounds := image.Rect(0, 0, 1920, 1080)
	gridInstance := grid.NewGrid(testCharacters, bounds, logger)

	if gridInstance.Characters() != testCharacters {
		t.Errorf("Characters() = %q, want %q", gridInstance.Characters(), testCharacters)
	}

	if gridInstance.Bounds() != bounds {
		t.Errorf("Bounds() = %v, want %v", gridInstance.Bounds(), bounds)
	}

	cells := gridInstance.Cells()
	if len(cells) == 0 {
		t.Error("Cells() returned empty slice")
	}

	index := gridInstance.Index()
	if len(index) != len(cells) {
		t.Errorf("Index() length = %d, want %d", len(index), len(cells))
	}

	allCells := gridInstance.AllCells()
	if len(allCells) != len(cells) {
		t.Errorf("AllCells() length = %d, want %d", len(allCells), len(cells))
	}
}

func TestCalculateOptimalGrid(t *testing.T) {
	tests := []struct {
		name       string
		characters string
		wantRows   int
		wantCols   int
	}{
		{"normal characters", testCharacters, 3, 3},
		{"empty string", "", 9, 9},
		{"single character", "A", 9, 9},
		{"long string", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", 26, 26},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			rows, cols := grid.CalculateOptimalGrid(testCase.characters)
			if rows != testCase.wantRows || cols != testCase.wantCols {
				t.Errorf("CalculateOptimalGrid(%q) = (%d, %d), want (%d, %d)",
					testCase.characters, rows, cols, testCase.wantRows, testCase.wantCols)
			}
		})
	}
}

func TestGrid_InvalidBounds(t *testing.T) {
	logger := logger.Get()

	// Test with zero width
	gridInstance := grid.NewGrid("ABC", image.Rect(0, 0, 0, 100), logger)
	if len(gridInstance.Cells()) != 0 {
		t.Error("Expected empty cells for zero width")
	}

	// Test with zero height
	gridInstance = grid.NewGrid("ABC", image.Rect(0, 0, 100, 0), logger)
	if len(gridInstance.Cells()) != 0 {
		t.Error("Expected empty cells for zero height")
	}

	// Test with negative dimensions (image.Rect normalizes this to positive dimensions)
	// So this actually creates a valid grid. Let's test with truly invalid bounds
	gridInstance = grid.NewGrid("ABC", image.Rect(100, 100, 100, 100), logger) // Zero width/height
	if len(gridInstance.Cells()) != 0 {
		t.Errorf(
			"Expected empty cells for zero dimensions, got %d cells",
			len(gridInstance.Cells()),
		)
	}
}

func TestGrid_EmptyCharacters(t *testing.T) {
	logger := logger.Get()
	bounds := image.Rect(0, 0, 300, 300)

	// Empty characters should default to alphabet
	gridInstance := grid.NewGrid("", bounds, logger)
	if gridInstance.Characters() != "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		t.Errorf(
			"Empty characters should default to alphabet, got %q",
			gridInstance.Characters(),
		)
	}

	// Single character should also default
	gridInstance = grid.NewGrid("A", bounds, logger)
	if gridInstance.Characters() != "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		t.Errorf(
			"Single character should default to alphabet, got %q",
			gridInstance.Characters(),
		)
	}
}

func TestGrid_WithCustomLabels(t *testing.T) {
	logger := logger.Get()
	bounds := image.Rect(0, 0, 300, 300)

	// Test with custom row and column labels
	gridInstance := grid.NewGridWithLabels(testCharacters, "123", "XYZ", bounds, logger)

	if gridInstance.Characters() != testCharacters {
		t.Errorf("Characters() = %q, want %q", gridInstance.Characters(), testCharacters)
	}

	// Check ValidCharacters includes all used characters
	validChars := gridInstance.ValidCharacters()

	expectedChars := "ABC123XYZ"
	for _, r := range expectedChars {
		if !strings.ContainsRune(validChars, r) {
			t.Errorf("ValidCharacters() missing %c, got %q", r, validChars)
		}
	}

	// Check that cells use the custom labels
	cells := gridInstance.Cells()
	if len(cells) == 0 {
		t.Fatal("No cells generated")
	}

	// Check for unique coordinates
	foundLabels := make(map[string]bool)
	for _, cell := range cells {
		if foundLabels[cell.Coordinate()] {
			t.Errorf("Duplicate coordinate found: %s", cell.Coordinate())
		}

		foundLabels[cell.Coordinate()] = true
	}

	// Should have unique coordinates for all cells
	if len(foundLabels) != len(cells) {
		t.Errorf("Expected %d unique coordinates, got %d", len(cells), len(foundLabels))
	}
}

func TestGrid_CustomLabelsWithSymbols(t *testing.T) {
	logger := logger.Get()
	bounds := image.Rect(0, 0, 500, 500)

	// Test with symbols in labels (like user's config)
	characters := "AOEUIDHTNSPYFGKXBM"
	rowLabels := "',.PYFGCRL/AOEUIDHTNS-;QJKXBMWVZ="
	colLabels := "AOEUIDHTNS"

	gridInstance := grid.NewGridWithLabels(characters, rowLabels, colLabels, bounds, logger)

	// Check ValidCharacters includes symbols
	validChars := gridInstance.ValidCharacters()

	expectedSymbols := "',./-;=QJKXBMWVZPYFGCRL"
	for _, r := range expectedSymbols {
		if !strings.ContainsRune(validChars, r) {
			t.Errorf("ValidCharacters() missing symbol %c, got %q", r, validChars)
		}
	}

	// Check cells have unique coordinates
	cells := gridInstance.Cells()

	coordMap := make(map[string]bool)
	for _, cell := range cells {
		coord := cell.Coordinate()
		if coordMap[coord] {
			t.Errorf("Duplicate coordinate: %s", coord)
		}

		coordMap[coord] = true

		// Verify coordinate uses valid characters
		for _, r := range coord {
			if !strings.ContainsRune(validChars, r) {
				t.Errorf("Coordinate %s contains invalid character %c", coord, r)
			}
		}
	}
}

func TestGrid_BackwardCompatibility(t *testing.T) {
	logger := logger.Get()
	bounds := image.Rect(0, 0, 300, 300)

	// Test that old NewGrid still works (empty row/col labels)
	gridInstance := grid.NewGrid(testCharacters, bounds, logger)

	if gridInstance.Characters() != testCharacters {
		t.Errorf("Characters() = %q, want %q", gridInstance.Characters(), testCharacters)
	}

	// ValidCharacters should be same as Characters when no custom labels
	if gridInstance.ValidCharacters() != gridInstance.Characters() {
		t.Errorf(
			"ValidCharacters() = %q, want %q",
			gridInstance.ValidCharacters(),
			gridInstance.Characters(),
		)
	}

	// Should have unique coordinates
	cells := gridInstance.Cells()

	coordMap := make(map[string]bool)
	for _, cell := range cells {
		if coordMap[cell.Coordinate()] {
			t.Errorf("Duplicate coordinate: %s", cell.Coordinate())
		}

		coordMap[cell.Coordinate()] = true
	}
}
