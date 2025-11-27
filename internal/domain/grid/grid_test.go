package grid_test

import (
	"image"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/infra/logger"
)

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

func TestPrewarm(t *testing.T) {
	// Test that Prewarm doesn't panic and creates cache entries
	sizes := []image.Rectangle{
		image.Rect(0, 0, 1920, 1080),
		image.Rect(0, 0, 1280, 720),
		image.Rect(0, 0, 800, 600),
	}

	// Should not panic
	grid.Prewarm("ABC", sizes)

	// Verify that grids were created and cached
	log := logger.Get()
	for _, size := range sizes {
		g := grid.NewGrid("ABC", size, log)
		if g == nil {
			t.Errorf("Prewarm should have created grid for size %v", size)
		}

		if len(g.AllCells()) == 0 {
			t.Errorf("Prewarm should have created cells for size %v", size)
		}
	}
}

func TestGrid_Cache(t *testing.T) {
	logger := logger.Get()
	characters := "ABC"
	bounds := image.Rect(0, 0, 300, 300)

	// Create first grid (should cache it)
	grid1 := grid.NewGrid(characters, bounds, logger)
	if grid1 == nil {
		t.Fatal("NewGrid returned nil")
	}

	cells1 := len(grid1.AllCells())

	// Create second grid with same parameters (should use cache)
	grid2 := grid.NewGrid(characters, bounds, logger)
	if grid2 == nil {
		t.Fatal("NewGrid returned nil")
	}

	cells2 := len(grid2.AllCells())

	if cells1 != cells2 {
		t.Errorf("Cached grid should have same number of cells: got %d, want %d", cells2, cells1)
	}

	// Verify they have the same cells
	if len(grid1.AllCells()) != len(grid2.AllCells()) {
		t.Error("Cached grids should have same cell count")
	}
}

func TestCell_Methods(t *testing.T) {
	logger := logger.Get()
	gridInstance := grid.NewGrid("ABC", image.Rect(0, 0, 300, 300), logger)
	cells := gridInstance.AllCells()

	if len(cells) == 0 {
		t.Fatal("No cells generated")
	}

	cell := cells[0]

	// Test Coordinate method
	coord := cell.Coordinate()
	if coord == "" {
		t.Error("Coordinate should not be empty")
	}

	if len(coord) != 4 {
		t.Errorf("Coordinate should be 4 characters, got %d", len(coord))
	}

	// Test Bounds method
	bounds := cell.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		t.Error("Bounds should have positive dimensions")
	}

	// Test Center method
	center := cell.Center()
	if center.X < 0 || center.Y < 0 {
		t.Error("Center should have non-negative coordinates")
	}

	// Verify center is within bounds
	if center.X < bounds.Min.X || center.X > bounds.Max.X ||
		center.Y < bounds.Min.Y || center.Y > bounds.Max.Y {
		t.Error("Center should be within bounds")
	}
}

func TestGrid_Getters(t *testing.T) {
	logger := logger.Get()
	bounds := image.Rect(0, 0, 1920, 1080)
	gridInstance := grid.NewGrid("ABC", bounds, logger)

	if gridInstance.Characters() != "ABC" {
		t.Errorf("Characters() = %q, want %q", gridInstance.Characters(), "ABC")
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
		{"normal characters", "ABC", 3, 3},
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
