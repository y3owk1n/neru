//go:build darwin

package accessibility

import (
	"context"
	"strings"
	"sync"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// StreamClickableNodes returns a channel that delivers clickable AXNodes as
// they are discovered during tree traversal, enabling true fluid streaming.
// The channel is closed when the stream is complete.
func (c *InfraAXClient) StreamClickableNodes(
	ctx context.Context,
	root AXElement,
	roles []string,
	maxDepth int,
) (<-chan AXNode, error) {
	element := c.extractElement(root)
	if element == nil {
		return nil, derrors.New(derrors.CodeInvalidInput, "element is nil")
	}

	nodeCh := make(chan AXNode, 100) //nolint:mnd // buffered channel for streaming throughput

	go c.streamClickableNodesGoroutine(ctx, element, roles, maxDepth, nodeCh)

	return nodeCh, nil
}

// streamClickableNodesGoroutine runs the streaming tree build in a background
// goroutine. Each clickable node is streamed through ch during tree traversal.
func (c *InfraAXClient) streamClickableNodesGoroutine(
	ctx context.Context,
	element *Element,
	roles []string,
	maxDepth int,
	nodeCh chan<- AXNode,
) {
	defer close(nodeCh)
	defer element.Release()

	opts, allowedRoles, ignoreClickableCheck := c.buildClickableOpts(element, roles, maxDepth)
	opts.SetSkipAccumulateSearchText(true)

	var keepMu sync.Mutex

	keepSet := make(map[*Element]struct{})

	opts.onNode = func(node *TreeNode) {
		if !node.element.IsClickable(
			node.info,
			allowedRoles,
			c.configProvider,
			ignoreClickableCheck,
		) {
			return
		}

		rect := rectFromInfo(node.info)
		if rect.Dx() == 0 || rect.Dy() == 0 {
			return
		}

		// Eagerly compute basic search text (title + desc + value) so that
		// streamed elements have search text available even though
		// accumulateSearchText hasn't run yet.
		if node.info.SearchText() == "" {
			var builder strings.Builder

			seen := make(map[string]struct{})
			appendSearchText(&builder, seen, node.info.Title())
			appendSearchText(&builder, seen, node.info.Description())
			appendSearchText(&builder, seen, node.info.Value())
			node.info.searchText = builder.String()
		}

		keepMu.Lock()
		keepSet[node.Element()] = struct{}{}
		keepMu.Unlock()

		select {
		case nodeCh <- &InfraNode{
			node:           node,
			clickable:      true,
			configProvider: c.configProvider,
		}:
		case <-ctx.Done():
		}
	}

	tree, treeErr := BuildTree(ctx, element, opts)
	if treeErr != nil {
		return
	}

	// Release all non-kept nodes. Streamed nodes are kept and ownership of
	// their underlying element is transferred to the channel consumer, which
	// must call Release on each received AXNode when done.
	keepMu.Lock()

	keepList := make([]*TreeNode, 0, len(keepSet))
	for elem := range keepSet {
		keepList = append(keepList, &TreeNode{element: elem})
	}
	keepMu.Unlock()

	releaseTreeExcept(tree, keepList)
}
