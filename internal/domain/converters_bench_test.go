package domain_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/domain"
)

func BenchmarkGetModeString(b *testing.B) {
	for b.Loop() {
		_ = domain.GetModeString(domain.ModeHints)
	}
}

func BenchmarkGetActionString(b *testing.B) {
	for b.Loop() {
		_ = domain.GetActionString(domain.ActionLeftClick)
	}
}

func BenchmarkGetActionFromString(b *testing.B) {
	for b.Loop() {
		_, _ = domain.GetActionFromString("left_click")
	}
}

func BenchmarkActionStringRoundTrip(b *testing.B) {
	for b.Loop() {
		str := domain.GetActionString(domain.ActionLeftClick)
		_, _ = domain.GetActionFromString(str)
	}
}
