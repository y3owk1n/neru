package grid_test

import (
	"image"
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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			grid := grid.NewGrid(test.chars, test.bounds, log)
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cell := grid.GetCellByCoordinate(test.coord)
			if (cell != nil) != test.want {
				t.Errorf(
					"GetCellByCoordinate(%q) exists = %v, want %v",
					test.coord,
					cell != nil,
					test.want,
				)
			}
		})
	}
}
