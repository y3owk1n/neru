package grid_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/grid"
	"go.uber.org/zap"
)

// BenchmarkNewGrid benchmarks grid creation.
func BenchmarkNewGrid(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	bounds := image.Rect(0, 0, 1920, 1080)
	characters := "asdfghjkl"

	b.ResetTimer()

	for b.Loop() {
		_ = grid.NewGrid(characters, bounds, logger)
	}
}

// BenchmarkGrid_CellByCoordinate benchmarks label-based lookup.
func BenchmarkGrid_CellByCoordinate(b *testing.B) {
	logger := zap.NewNop()
	grid := grid.NewGrid("abcdefghijklmnopqrstuvwxyz", image.Rect(0, 0, 1920, 1080), logger)

	b.ResetTimer()
	// The instruction "for range b.N" is not valid Go syntax for iterating N times.
	// The standard and correct way is "for i := 0; i < b.N; i++".
	// Adhering to "syntactically correct" requirement, using the standard loop.
	for b.Loop() {
		_ = grid.CellByCoordinate("aa")
	}
}

// BenchmarkGrid_AllCells benchmarks getting all cells.
func BenchmarkGrid_AllCells(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	bounds := image.Rect(0, 0, 1920, 1080)
	g := grid.NewGrid("asdfghjkl", bounds, logger)

	b.ResetTimer()

	for b.Loop() {
		_ = g.AllCells()
	}
}

func BenchmarkGrid_CellByCoordinate_Miss(b *testing.B) {
	logger := zap.NewNop()
	grid := grid.NewGrid("abcdefghijklmnopqrstuvwxyz", image.Rect(0, 0, 1920, 1080), logger)

	b.ResetTimer()

	for b.Loop() {
		grid.CellByCoordinate("zz")
	}
}
