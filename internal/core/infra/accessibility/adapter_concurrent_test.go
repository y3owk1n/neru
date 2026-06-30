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

const (
	testID       = "test-id"
	axButtonRole = "AXButton"
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
			MockID:     testID,
			MockBounds: image.Rect(0, 0, 100, 100),
			MockRole:   axButtonRole,
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

// holdNode blocks on Bounds() until released, guaranteeing a worker is
// mid-processing when we cancel the context.
type holdNode struct {
	MockNode

	started chan struct{} // closed on first Bounds() call
	release chan struct{} // close to unblock
	once    sync.Once
}

func (n *holdNode) Bounds() image.Rectangle {
	n.once.Do(func() { close(n.started) })

	<-n.release

	return n.MockBounds
}

func TestProcessClickableNodesConcurrent_CancelledMidProcessing(t *testing.T) {
	logger := zap.NewNop()
	adapter := NewAdapter(logger, nil, nil, &MockAXClient{}, false)

	nodeCount := ConcurrentProcessingThreshold * 4
	started := make(chan struct{})
	release := make(chan struct{})

	// First node is a holdNode that blocks until we release it, ensuring
	// the worker is still running when we cancel the context.
	baseNode := &MockNode{
		MockID:     "test-id",
		MockBounds: image.Rect(0, 0, 100, 100),
		MockRole:   axButtonRole,
	}

	nodes := make([]AXNode, nodeCount)

	nodes[0] = &holdNode{MockNode: *baseNode, started: started, release: release}
	for i := 1; i < nodeCount; i++ {
		nodes[i] = &MockNode{
			MockID:     testID,
			MockBounds: image.Rect(0, 0, 100, 100),
			MockRole:   axButtonRole,
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

	// Wait for a worker to begin processing a node, then cancel mid-flight
	// while the worker is still blocked on Bounds().
	<-started
	cancel()

	// Release the blocked worker so it can observe the canceled context.
	close(release)

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
			MockID:     testID,
			MockBounds: image.Rect(0, 0, 100, 100),
			MockRole:   axButtonRole,
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
			MockID:     testID,
			MockBounds: image.Rect(0, 0, 100, 100),
			MockRole:   axButtonRole,
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
