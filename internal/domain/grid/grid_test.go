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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := grid.NewGrid(tt.chars, tt.bounds, log)
			if len(g.GetAllCells()) == 0 {
				t.Error("Expected cells to be generated")
			}
		})
	}
}

func TestGrid_GetCellByCoordinate(t *testing.T) {
	log := logger.Get()
	g := grid.NewGrid("ABC", image.Rect(0, 0, 300, 300), log)

	// Get a valid coordinate from the generated grid
	cells := g.GetAllCells()
	if len(cells) == 0 {
		t.Fatal("Expected cells to be generated")
	}
	validCoord := cells[0].GetCoordinate()

	tests := []struct {
		name  string
		coord string
		want  bool // exists
	}{
		{"valid " + validCoord, validCoord, true},
		{"invalid ZZZ", "ZZZ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cell := g.GetCellByCoordinate(tt.coord)
			if (cell != nil) != tt.want {
				t.Errorf(
					"GetCellByCoordinate(%q) exists = %v, want %v",
					tt.coord,
					cell != nil,
					tt.want,
				)
			}
		})
	}
}
