//go:build darwin

package darwin_test

import (
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
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
