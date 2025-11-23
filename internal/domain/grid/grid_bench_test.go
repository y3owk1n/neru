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
	for i := 0; i < b.N; i++ {
		_ = NewGrid(characters, bounds, logger)
	}
}

// BenchmarkGrid_GetCellByCoordinate benchmarks label-based lookup.
func BenchmarkGrid_GetCellByCoordinate(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	bounds := image.Rect(0, 0, 1920, 1080)
	g := NewGrid("asdfghjkl", bounds, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.GetCellByCoordinate("AAA")
	}
}

// BenchmarkGrid_GetAllCells benchmarks getting all cells.
func BenchmarkGrid_GetAllCells(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	bounds := image.Rect(0, 0, 1920, 1080)
	g := NewGrid("asdfghjkl", bounds, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.GetAllCells()
	}
}
