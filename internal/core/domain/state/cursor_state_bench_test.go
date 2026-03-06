package state_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/state"
)

func BenchmarkCursorState_Capture(b *testing.B) {
	state := state.NewCursorState()
	pos := image.Point{X: 100, Y: 200}
	bounds := image.Rect(0, 0, 1920, 1080)

	for b.Loop() {
		state.Capture(pos, bounds)
	}
}

func BenchmarkCursorState_ShouldMoveCursor(b *testing.B) {
	state := state.NewCursorState()
	state.Capture(image.Point{X: 100, Y: 200}, image.Rect(0, 0, 1920, 1080))

	for b.Loop() {
		_ = state.ShouldMoveCursor()
	}
}

func BenchmarkCursorState_ConcurrentAccess(b *testing.B) {
	state := state.NewCursorState()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			state.Capture(image.Point{X: 100, Y: 200}, image.Rect(0, 0, 1920, 1080))
			_ = state.ShouldMoveCursor()
		}
	})
}
