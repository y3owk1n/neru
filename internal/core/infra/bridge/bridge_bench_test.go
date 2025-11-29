//go:build !integration

package bridge_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/bridge"
	"go.uber.org/zap"
)

func BenchmarkActiveScreenBounds(b *testing.B) {
	bridge.InitializeLogger(zap.NewNop())

	for b.Loop() {
		_ = bridge.ActiveScreenBounds()
	}
}

func BenchmarkHasClickAction(b *testing.B) {
	bridge.InitializeLogger(zap.NewNop())

	for b.Loop() {
		_ = bridge.HasClickAction(nil)
	}
}
