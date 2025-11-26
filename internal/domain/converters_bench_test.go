package domain_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/domain"
)

func BenchmarkModeString(b *testing.B) {
	for b.Loop() {
		_ = domain.ModeString(domain.ModeHints)
	}
}

func BenchmarkActionString(b *testing.B) {
	for b.Loop() {
		_ = domain.ActionString(domain.ActionLeftClick)
	}
}

func BenchmarkActionFromString(b *testing.B) {
	for b.Loop() {
		_, _ = domain.ActionFromString("left_click")
	}
}

func BenchmarkActionStringRoundTrip(b *testing.B) {
	for b.Loop() {
		str := domain.ActionString(domain.ActionLeftClick)
		_, _ = domain.ActionFromString(str)
	}
}
