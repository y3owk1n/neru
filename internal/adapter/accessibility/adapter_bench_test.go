package accessibility_test

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/accessibility"
	"go.uber.org/zap"
)

func BenchmarkGetScreenBounds(b *testing.B) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{}
	adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)
	ctx := context.Background()

	for b.Loop() {
		_, _ = adapter.GetScreenBounds(ctx)
	}
}

func BenchmarkGetCursorPosition(b *testing.B) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{}
	adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)
	ctx := context.Background()

	for b.Loop() {
		_, _ = adapter.GetCursorPosition(ctx)
	}
}

func BenchmarkIsAppExcluded(b *testing.B) {
	logger := zap.NewNop()
	excludedBundles := []string{"com.apple.finder", "com.apple.dock"}
	mockClient := &accessibility.MockAXClient{}
	adapter := accessibility.NewAdapter(logger, excludedBundles, []string{}, mockClient)
	ctx := context.Background()

	for b.Loop() {
		_ = adapter.IsAppExcluded(ctx, "com.google.Chrome")
	}
}
