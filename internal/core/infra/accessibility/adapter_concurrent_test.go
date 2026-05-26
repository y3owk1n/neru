//nolint:testpackage
package accessibility

import (
	"context"
	"image"
	"sync"
	"testing"

	"go.uber.org/goleak"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain/element"
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

// signalNode wraps MockNode and closes the started channel on the first Bounds() call,
// providing a deterministic handshake that a worker has begun processing a node.
type signalNode struct {
	MockNode

	started chan struct{}
	once    sync.Once
}

func (n *signalNode) Bounds() image.Rectangle {
	n.once.Do(func() { close(n.started) })

	return n.MockBounds
}

func TestProcessClickableNodesConcurrent_CancelledMidProcessing(t *testing.T) {
	logger := zap.NewNop()
	adapter := NewAdapter(logger, nil, nil, &MockAXClient{}, false)

	nodeCount := ConcurrentProcessingThreshold * 4
	started := make(chan struct{})

	// First node is a signalNode so we know when a worker starts processing
	// after the context check passes at idx=0.
	baseNode := &MockNode{
		MockID:     "test-id",
		MockBounds: image.Rect(0, 0, 100, 100),
		MockRole:   "AXButton",
	}

	nodes := make([]AXNode, nodeCount)

	nodes[0] = &signalNode{MockNode: *baseNode, started: started}
	for i := 1; i < nodeCount; i++ {
		nodes[i] = &MockNode{
			MockID:     "test-id",
			MockBounds: image.Rect(0, 0, 100, 100),
			MockRole:   "AXButton",
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	var (
		result []*element.Element
		err    error
	)

	done := make(chan struct{})
	go func() {
		result, err = adapter.processClickableNodesConcurrent(ctx, nodes, ports.ElementFilter{})

		close(done)
	}()

	// Wait for a worker to begin processing a node, then cancel mid-flight.
	<-started
	cancel()

	<-done

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
