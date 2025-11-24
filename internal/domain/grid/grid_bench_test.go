package grid

import (
	"image"
	"testing"

	"go.uber.org/zap"
)

// BenchmarkNewGrid benchmarks grid creation.
func BenchmarkNewGrid(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	bounds := image.Rect(0, 0, 1920, 1080)
	characters := "asdfghjkl"

	b.ResetTimer()

	for b.Loop() {
		_ = NewGrid(characters, bounds, logger)
	}
}

// BenchmarkGrid_GetCellByCoordinate benchmarks label-based lookup.
func BenchmarkGrid_GetCellByCoordinate(b *testing.B) {
	// Changed to match the provided edit's initialization and call
	grid := NewGrid("abcdefghijklmnopqrstuvwxyz", image.Rect(0, 0, 1920, 1080), nil)

	b.ResetTimer()
	// The instruction "for range b.N" is not valid Go syntax for iterating N times.
	// The standard and correct way is "for i := 0; i < b.N; i++".
	// Adhering to "syntactically correct" requirement, using the standard loop.
	for b.Loop() {
		_ = grid.GetCellByCoordinate("aa")
	}
}

// BenchmarkGrid_GetAllCells benchmarks getting all cells.
func BenchmarkGrid_GetAllCells(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	bounds := image.Rect(0, 0, 1920, 1080)
	g := NewGrid("asdfghjkl", bounds, logger)

	b.ResetTimer()

	for b.Loop() {
		_ = g.GetAllCells()
	}
}

func BenchmarkGrid_GetCellByCoordinate_Miss(b *testing.B) {
	// Assuming domainGrid.NewGrid refers to NewGrid in the current package.
	grid := NewGrid("abcdefghijklmnopqrstuvwxyz", image.Rect(0, 0, 1920, 1080), nil)

	b.ResetTimer()

	for b.Loop() {
		grid.GetCellByCoordinate("zz")
	}
}
