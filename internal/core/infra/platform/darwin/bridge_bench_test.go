//go:build darwin

package darwin_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
	"go.uber.org/zap"
)

func BenchmarkActiveScreenBounds(b *testing.B) {
	darwin.InitializeLogger(zap.NewNop())

	for b.Loop() {
		_ = darwin.ActiveScreenBounds()
	}
}

func BenchmarkHasClickAction(b *testing.B) {
	darwin.InitializeLogger(zap.NewNop())

	for b.Loop() {
		_ = darwin.HasClickAction(nil)
	}
}
