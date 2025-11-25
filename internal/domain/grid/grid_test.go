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
			if len(grid.GetAllCells()) == 0 {
				t.Error("Expected cells to be generated")
			}
		})
	}
}

func TestGrid_GetCellByCoordinate(t *testing.T) {
	logger := logger.Get()
	grid := grid.NewGrid("ABC", image.Rect(0, 0, 300, 300), logger)

	// Get a valid coordinate from the generated grid
	cells := grid.GetAllCells()
	if len(cells) == 0 {
		t.Fatal("Expected cells to be generated")
	}

	validCoordinate := cells[0].GetCoordinate()

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
			cell := grid.GetCellByCoordinate(testCase.coord)
			if (cell != nil) != testCase.want {
				t.Errorf(
					"GetCellByCoordinate(%q) exists = %v, want %v",
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

	cells := grid.GetAllCells()
	if len(cells) == 0 {
		t.Fatal("Expected cells to be generated")
	}

	cell := cells[0]

	// Test that methods return non-zero values
	if cell.GetCoordinate() == "" {
		t.Error("GetCoordinate() returned empty string")
	}

	bounds := cell.GetBounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		t.Errorf("GetBounds() returned invalid bounds: %v", bounds)
	}

	center := cell.GetCenter()
	if center.X < 0 || center.Y < 0 {
		t.Errorf("GetCenter() returned invalid center: %v", center)
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

	if gridInstance.GetCharacters() != "ABC" {
		t.Errorf("GetCharacters() = %q, want %q", gridInstance.GetCharacters(), "ABC")
	}

	if gridInstance.GetBounds() != bounds {
		t.Errorf("GetBounds() = %v, want %v", gridInstance.GetBounds(), bounds)
	}

	cells := gridInstance.GetCells()
	if len(cells) == 0 {
		t.Error("GetCells() returned empty slice")
	}

	index := gridInstance.GetIndex()
	if len(index) != len(cells) {
		t.Errorf("GetIndex() length = %d, want %d", len(index), len(cells))
	}

	allCells := gridInstance.GetAllCells()
	if len(allCells) != len(cells) {
		t.Errorf("GetAllCells() length = %d, want %d", len(allCells), len(cells))
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
	if len(gridInstance.GetCells()) != 0 {
		t.Error("Expected empty cells for zero width")
	}

	// Test with zero height
	gridInstance = grid.NewGrid("ABC", image.Rect(0, 0, 100, 0), logger)
	if len(gridInstance.GetCells()) != 0 {
		t.Error("Expected empty cells for zero height")
	}

	// Test with negative dimensions (image.Rect normalizes this to positive dimensions)
	// So this actually creates a valid grid. Let's test with truly invalid bounds
	gridInstance = grid.NewGrid("ABC", image.Rect(100, 100, 100, 100), logger) // Zero width/height
	if len(gridInstance.GetCells()) != 0 {
		t.Errorf(
			"Expected empty cells for zero dimensions, got %d cells",
			len(gridInstance.GetCells()),
		)
	}
}

func TestGrid_EmptyCharacters(t *testing.T) {
	logger := logger.Get()
	bounds := image.Rect(0, 0, 300, 300)

	// Empty characters should default to alphabet
	gridInstance := grid.NewGrid("", bounds, logger)
	if gridInstance.GetCharacters() != "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		t.Errorf(
			"Empty characters should default to alphabet, got %q",
			gridInstance.GetCharacters(),
		)
	}

	// Single character should also default
	gridInstance = grid.NewGrid("A", bounds, logger)
	if gridInstance.GetCharacters() != "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		t.Errorf(
			"Single character should default to alphabet, got %q",
			gridInstance.GetCharacters(),
		)
	}
}
