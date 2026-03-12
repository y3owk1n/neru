//go:build linux

package accessibility

import (
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
)

// TreeNode represents a node in the accessibility element hierarchy (Linux stub).
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
func (n *TreeNode) FindClickableElements(keptRoles map[string]struct{}, cache any) []*TreeNode {
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

// SetIncludeOutOfBounds is a Linux stub.
func (o *TreeOptions) SetIncludeOutOfBounds(include bool) {}

// SetMaxDepth is a Linux stub.
func (o *TreeOptions) SetMaxDepth(depth int) {}

// SetParallelThreshold is a Linux stub.
func (o *TreeOptions) SetParallelThreshold(threshold int) {}

// BuildTree builds the accessibility tree for the specified root element (Linux stub).
func BuildTree(_ *Element, _ TreeOptions) (*TreeNode, error) {
	return &TreeNode{}, nil
}

// ProcessClickableNodes processes the clickable nodes in the tree (Linux stub).
func ProcessClickableNodes(root *TreeNode, cfg config.HintsConfig) []*TreeNode {
	return nil
}

// ReleaseTree releases the tree and its nodes to the pool (Linux stub).
func ReleaseTree(root *TreeNode) {}
