package bridge_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/infra/bridge"
	"go.uber.org/zap"
)

func BenchmarkGetActiveScreenBounds(b *testing.B) {
	bridge.InitializeLogger(zap.NewNop())

	for b.Loop() {
		_ = bridge.GetActiveScreenBounds()
	}
}

func BenchmarkHasClickAction(b *testing.B) {
	bridge.InitializeLogger(zap.NewNop())

	for b.Loop() {
		_ = bridge.HasClickAction(nil)
	}
}
