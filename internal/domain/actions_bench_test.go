package domain_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/domain"
)

func BenchmarkKnownActionNames(b *testing.B) {
	for b.Loop() {
		_ = domain.KnownActionNames()
	}
}

func BenchmarkIsKnownActionName(b *testing.B) {
	for b.Loop() {
		_ = domain.IsKnownActionName(domain.ActionNameLeftClick)
	}
}

func BenchmarkIsKnownActionName_Unknown(b *testing.B) {
	for b.Loop() {
		_ = domain.IsKnownActionName(domain.ActionName("unknown"))
	}
}
