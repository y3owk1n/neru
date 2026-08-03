//go:build linux

package linux

import (
	"context"
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
)

// TreeNode is a node in the accessibility element hierarchy (Linux stub).
type TreeNode struct{}

// Element returns the node's element (Linux stub).
func (n *TreeNode) Element() *Element { return nil }

// Info returns the node's info (Linux stub).
func (n *TreeNode) Info() *ElementInfo { return nil }

// Children returns the node's children (Linux stub).
func (n *TreeNode) Children() []*TreeNode { return nil }

// Parent returns the node's parent (Linux stub).
func (n *TreeNode) Parent() *TreeNode { return nil }

// FindClickableElements is a Linux stub.
func (n *TreeNode) FindClickableElements(
	keptRoles map[string]struct{},
	configProvider config.Provider,
	ignoreClickableCheck bool,
) []*TreeNode {
	return nil
}

// Release is a Linux stub.
func (n *TreeNode) Release(keep map[*Element]struct{}) {}

// TreeOptions defines options for tree building (Linux stub).
type TreeOptions struct {
	MaxDepth int
	Bounds   image.Rectangle
}

// DefaultTreeOptions returns the default tree options (Linux stub).
func DefaultTreeOptions(logger *zap.Logger) TreeOptions { return TreeOptions{} }

// SetCache is a Linux stub.
func (o *TreeOptions) SetCache(cache any) {}

// SetMaxDepth is a Linux stub.
func (o *TreeOptions) SetMaxDepth(depth int) {}

// SetBundleID is a Linux stub.
func (o *TreeOptions) SetBundleID(bundleID string) {}

// SetConfigProvider is a Linux stub.
func (o *TreeOptions) SetConfigProvider(cp config.Provider) {}

// SetFilterFunc is a Linux stub.
func (o *TreeOptions) SetFilterFunc(fn func(*ElementInfo) bool) {}

// SetRoles is a Linux stub: the AT-SPI client receives the role set directly.
func (o *TreeOptions) SetRoles(roles map[string]struct{}) {}

// BuildTree builds the accessibility tree for the specified root element (Linux stub).
func BuildTree(_ context.Context, _ *Element, _ TreeOptions) (*TreeNode, error) {
	return &TreeNode{}, nil
}

// ProcessClickableNodes processes the clickable nodes in the tree (Linux stub).
func ProcessClickableNodes(root *TreeNode, cfg config.HintsConfig) []*TreeNode {
	return nil
}

// ReleaseTree releases the tree and its nodes to the pool (Linux stub).
func ReleaseTree(root *TreeNode) {}

// ReleaseTreeExcept releases all AXUIElementRefs in the tree except those
// belonging to the keep list. This prevents leaking CFRetain'd refs from
// NeruGetChildren/NeruGetVisibleRows that are stored in tree nodes but never returned
// to callers.
func ReleaseTreeExcept(tree *TreeNode, keep []*TreeNode) {
	keepSet := make(map[*Element]struct{}, len(keep))
	for _, node := range keep {
		if node.Element() != nil {
			keepSet[node.Element()] = struct{}{}
		}
	}

	tree.Release(keepSet)
}
