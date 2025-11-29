//go:build !integration

package grid_test

import (
	"image"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/grid"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
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
