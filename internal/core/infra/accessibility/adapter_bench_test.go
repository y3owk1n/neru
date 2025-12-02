package accessibility_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/accessibility"
	"github.com/y3owk1n/neru/internal/core/ports"
	"go.uber.org/zap"
)

func BenchmarkScreenBounds(b *testing.B) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{}
	adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)
	ctx := context.Background()

	for b.Loop() {
		_, _ = adapter.ScreenBounds(ctx)
	}
}

func BenchmarkCursorPosition(b *testing.B) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{}
	adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)
	ctx := context.Background()

	for b.Loop() {
		_, _ = adapter.CursorPosition(ctx)
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

func BenchmarkClickableElements_Concurrent(b *testing.B) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{
		MockFrontmostWindow: &accessibility.MockWindow{},
	}

	// Create enough nodes to exceed ConcurrentProcessingThreshold for concurrent processing
	numNodes := accessibility.ConcurrentProcessingThreshold * 10

	nodes := make([]accessibility.AXNode, numNodes)
	for i := range numNodes {
		nodes[i] = &accessibility.MockNode{
			MockID:     "node",
			MockRole:   "AXButton",
			MockBounds: image.Rect(0, 0, 100, 100),
		}
	}

	mockClient.MockClickableNodes = nodes

	adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)
	ctx := context.Background()
	filter := ports.DefaultElementFilter()

	b.ResetTimer()

	for b.Loop() {
		_, _ = adapter.ClickableElements(ctx, filter)
	}
}

func BenchmarkClickableElements_Sequential(b *testing.B) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{
		MockFrontmostWindow: &accessibility.MockWindow{},
	}

	// Create fewer nodes to stay below ConcurrentProcessingThreshold for sequential processing
	numNodes := accessibility.ConcurrentProcessingThreshold / 2

	nodes := make([]accessibility.AXNode, numNodes)
	for i := range numNodes {
		nodes[i] = &accessibility.MockNode{
			MockID:     "node",
			MockRole:   "AXButton",
			MockBounds: image.Rect(0, 0, 100, 100),
		}
	}

	mockClient.MockClickableNodes = nodes

	adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)
	ctx := context.Background()
	filter := ports.DefaultElementFilter()

	b.ResetTimer()

	for b.Loop() {
		_, _ = adapter.ClickableElements(ctx, filter)
	}
}
