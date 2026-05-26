//nolint:testpackage
package accessibility

import (
	"context"
	"image"
	"testing"

	"go.uber.org/goleak"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/ports"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestProcessClickableNodesConcurrent_CancelledBeforeStart(t *testing.T) {
	logger := zap.NewNop()
	adapter := NewAdapter(logger, nil, nil, &MockAXClient{}, false)

	nodeCount := ConcurrentProcessingThreshold + 50

	nodes := make([]AXNode, nodeCount)
	for i := range nodeCount {
		nodes[i] = &MockNode{
			MockID:     "test-id",
			MockBounds: image.Rect(0, 0, 100, 100),
			MockRole:   "AXButton",
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := adapter.processClickableNodesConcurrent(ctx, nodes, ports.ElementFilter{})
	if err == nil {
		t.Fatal("expected error from canceled context, got nil")
	}

	if result != nil {
		t.Fatal("expected nil result on cancellation, got non-nil slice")
	}
}

func TestProcessClickableNodesConcurrent_CancelledMidProcessing(t *testing.T) {
	logger := zap.NewNop()
	adapter := NewAdapter(logger, nil, nil, &MockAXClient{}, false)

	// Use enough nodes that workers are busy when cancellation arrives
	nodeCount := ConcurrentProcessingThreshold * 4

	nodes := make([]AXNode, nodeCount)
	for i := range nodeCount {
		nodes[i] = &MockNode{
			MockID:     "test-id",
			MockBounds: image.Rect(0, 0, 100, 100),
			MockRole:   "AXButton",
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := adapter.processClickableNodesConcurrent(ctx, nodes, ports.ElementFilter{})
	if err == nil {
		t.Fatal("expected error from canceled context, got nil")
	}

	if result != nil {
		t.Fatal("expected nil result on cancellation, got non-nil slice")
	}
}

func TestProcessClickableNodesConcurrent_HappyPath(t *testing.T) {
	logger := zap.NewNop()
	adapter := NewAdapter(logger, nil, nil, &MockAXClient{}, false)

	nodeCount := ConcurrentProcessingThreshold + 50

	nodes := make([]AXNode, nodeCount)
	for i := range nodeCount {
		nodes[i] = &MockNode{
			MockID:     "test-id",
			MockBounds: image.Rect(0, 0, 100, 100),
			MockRole:   "AXButton",
		}
	}

	ctx := context.Background()

	result, err := adapter.processClickableNodesConcurrent(ctx, nodes, ports.ElementFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != nodeCount {
		t.Fatalf("expected %d elements, got %d", nodeCount, len(result))
	}
}

func TestProcessClickableNodesConcurrent_BelowThreshold(t *testing.T) {
	logger := zap.NewNop()
	adapter := NewAdapter(logger, nil, nil, &MockAXClient{}, false)

	// Below the concurrent processing threshold — tests the sequential path
	nodeCount := ConcurrentProcessingThreshold - 10

	nodes := make([]AXNode, nodeCount)
	for i := range nodeCount {
		nodes[i] = &MockNode{
			MockID:     "test-id",
			MockBounds: image.Rect(0, 0, 100, 100),
			MockRole:   "AXButton",
		}
	}

	ctx := context.Background()

	result, err := adapter.processClickableNodes(ctx, nodes, ports.ElementFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != nodeCount {
		t.Fatalf("expected %d elements, got %d", nodeCount, len(result))
	}
}
