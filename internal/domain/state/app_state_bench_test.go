package state_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
)

func BenchmarkAppState_GetSet(b *testing.B) {
	state := state.NewAppState()

	for b.Loop() {
		state.SetEnabled(true)
		_ = state.IsEnabled()
		state.SetMode(domain.ModeHints)
		_ = state.CurrentMode()
	}
}

func BenchmarkAppState_ConcurrentAccess(b *testing.B) {
	state := state.NewAppState()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			state.SetEnabled(true)
			_ = state.IsEnabled()
		}
	})
}
