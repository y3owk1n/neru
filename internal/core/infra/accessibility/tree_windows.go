//go:build windows

package accessibility

import (
	"image"

	"github.com/y3owk1n/neru/internal/config"
	"go.uber.org/zap"
)

// TreeNode represents a node in the accessibility element hierarchy (Windows stub).
type TreeNode struct{}

// Element returns the node's element (Windows stub).
func (n *TreeNode) Element() *Element { return nil }

// Info returns the node's info (Windows stub).
func (n *TreeNode) Info() *ElementInfo { return nil }

// Children returns the node's children (Windows stub).
func (n *TreeNode) Children() []*TreeNode { return nil }

// Parent returns the node's parent (Windows stub).
func (n *TreeNode) Parent() *TreeNode { return nil }

// FindClickableElements is a Windows stub.
func (n *TreeNode) FindClickableElements(keptRoles map[string]struct{}, cache any) []*TreeNode {
	return nil
}

// Release is a Windows stub.
func (n *TreeNode) Release(keep map[*Element]struct{}) {}

// TreeOptions defines options for tree building (Windows stub).
type TreeOptions struct {
	MaxDepth int
	Bounds   image.Rectangle
}

// DefaultTreeOptions returns the default tree options (Windows stub).
func DefaultTreeOptions(logger *zap.Logger) TreeOptions { return TreeOptions{} }

// SetCache is a Windows stub.
func (o *TreeOptions) SetCache(cache any) {}

// SetIncludeOutOfBounds is a Windows stub.
func (o *TreeOptions) SetIncludeOutOfBounds(include bool) {}

// SetMaxDepth is a Windows stub.
func (o *TreeOptions) SetMaxDepth(depth int) {}

// SetParallelThreshold is a Windows stub.
func (o *TreeOptions) SetParallelThreshold(threshold int) {}

// BuildTree builds the accessibility tree for the specified root element (Windows stub).
func BuildTree(root *Element, opts TreeOptions) (*TreeNode, error) {
	return nil, nil
}

// ProcessClickableNodes processes the clickable nodes in the tree (Windows stub).
func ProcessClickableNodes(root *TreeNode, cfg config.HintsConfig) []*TreeNode {
	return nil
}

// ReleaseTree releases the tree and its nodes to the pool (Windows stub).
func ReleaseTree(root *TreeNode) {}
