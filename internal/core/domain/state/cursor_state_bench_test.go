//go:build unit

package state_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/state"
)

func BenchmarkCursorState_Capture(b *testing.B) {
	state := state.NewCursorState(true)
	pos := image.Point{X: 100, Y: 200}
	bounds := image.Rect(0, 0, 1920, 1080)

	for b.Loop() {
		state.Capture(pos, bounds)
	}
}

func BenchmarkCursorState_ShouldRestore(b *testing.B) {
	state := state.NewCursorState(true)
	state.Capture(image.Point{X: 100, Y: 200}, image.Rect(0, 0, 1920, 1080))

	for b.Loop() {
		_ = state.ShouldRestore()
	}
}

func BenchmarkCursorState_ConcurrentAccess(b *testing.B) {
	state := state.NewCursorState(true)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			state.Capture(image.Point{X: 100, Y: 200}, image.Rect(0, 0, 1920, 1080))
			_ = state.ShouldRestore()
		}
	})
}
