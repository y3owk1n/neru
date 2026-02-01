package domain_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
)

func BenchmarkModeString(b *testing.B) {
	for b.Loop() {
		_ = domain.ModeString(domain.ModeHints)
	}
}

func BenchmarkActionString(b *testing.B) {
	for b.Loop() {
		_ = domain.ActionString(action.TypeLeftClick)
	}
}

func BenchmarkActionFromString(b *testing.B) {
	for b.Loop() {
		_, _ = domain.ActionFromString("left_click")
	}
}

func BenchmarkActionStringRoundTrip(b *testing.B) {
	for b.Loop() {
		str := domain.ActionString(action.TypeLeftClick)
		_, _ = domain.ActionFromString(str)
	}
}
