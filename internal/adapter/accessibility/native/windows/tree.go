//go:build windows

package windows

import (
	"context"
	"image"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// TreeNode is a node in the accessibility element hierarchy. On Windows the
// tree is flat: a root window node whose children are the clickable controls
// UI Automation found at any depth below the window, in one cached query.
// Each node owns a unique *Element so callers can use the pointer identity as
// a stable element ID.
type TreeNode struct {
	element  *Element
	info     *ElementInfo
	children []*TreeNode
}

// Element returns the node's element.
func (n *TreeNode) Element() *Element {
	if n == nil {
		return nil
	}

	return n.element
}

// Info returns the node's info.
func (n *TreeNode) Info() *ElementInfo {
	if n == nil {
		return nil
	}

	return n.info
}

// Children returns the node's children.
func (n *TreeNode) Children() []*TreeNode {
	if n == nil {
		return nil
	}

	return n.children
}

// Parent returns the node's parent. The Windows tree is not back-linked.
func (n *TreeNode) Parent() *TreeNode { return nil }

// FindClickableElements returns the clickable descendant nodes. When keptRoles
// is non-empty, only nodes whose role is in the set are returned.
func (n *TreeNode) FindClickableElements(
	keptRoles map[string]struct{},
	_ config.Provider,
	_ bool,
) []*TreeNode {
	if n == nil {
		return nil
	}

	var out []*TreeNode

	var walk func(node *TreeNode)

	walk = func(node *TreeNode) {
		if node == nil {
			return
		}

		if node.info != nil && node.info.clickable {
			if _, ok := keptRoles[node.info.role]; ok || len(keptRoles) == 0 {
				out = append(out, node)
			}
		}

		for _, child := range node.children {
			walk(child)
		}
	}

	for _, child := range n.children {
		walk(child)
	}

	return out
}

// Release is a no-op: Windows nodes hold no live COM references.
func (n *TreeNode) Release(_ map[*Element]struct{}) {}

// TreeOptions defines options for tree building.
type TreeOptions struct {
	MaxDepth int
	Bounds   image.Rectangle
	logger   *zap.Logger
	// Roles is the set of UIA control-type names to enumerate. An empty set
	// falls back to the shipped defaults. It is applied during enumeration so
	// unwanted controls are rejected before their properties are read.
	Roles map[string]struct{}
}

// DefaultTreeOptions returns the default tree options.
func DefaultTreeOptions(logger *zap.Logger) TreeOptions {
	if logger == nil {
		logger = zap.NewNop()
	}

	return TreeOptions{logger: logger}
}

// SetCache is a no-op on Windows: the UIA cache request is built inside every
// enumeration rather than handed in by the caller.
func (o *TreeOptions) SetCache(_ any) {}

// SetMaxDepth records the maximum tree depth. The UIA query has no depth
// knob (TreeScope_Descendants is all or nothing), so the value is recorded and
// not applied; hints.max_depth is declared inert on Windows for that reason.
func (o *TreeOptions) SetMaxDepth(depth int) { o.MaxDepth = depth }

// SetBundleID is a no-op on Windows.
func (o *TreeOptions) SetBundleID(_ string) {}

// SetConfigProvider is a no-op on Windows.
func (o *TreeOptions) SetConfigProvider(_ config.Provider) {}

// SetFilterFunc is a no-op on Windows.
func (o *TreeOptions) SetFilterFunc(_ func(*ElementInfo) bool) {}

// SetRoles records the UIA control-type names to enumerate.
func (o *TreeOptions) SetRoles(roles map[string]struct{}) { o.Roles = roles }

// BuildTree enumerates the clickable controls under the given window element
// via UI Automation and returns a root node with one child per control. For
// non-window elements (no HWND) it returns an empty root.
//
// The UIA query is one blocking COM call and cannot be interrupted, so the
// context is honored at its edges: a deadline that has already passed skips
// the query, and one that passes during it discards the result rather than
// handing stale elements to a caller that has stopped waiting.
func BuildTree(ctx context.Context, root *Element, opts TreeOptions) (*TreeNode, error) {
	if root == nil {
		return &TreeNode{}, nil
	}

	rootInfo, _ := root.Info()

	rootNode := &TreeNode{
		element: root,
		info:    rootInfo,
	}

	if root.hwnd == 0 {
		return rootNode, nil
	}

	if ctx.Err() != nil {
		return nil, derrors.WrapContextCanceled(ctx, "UIA tree build")
	}

	started := time.Now()

	controls := enumerateClickableElements(root.hwnd, opts.Roles)

	if ctx.Err() != nil {
		return nil, derrors.WrapContextCanceled(ctx, "UIA tree build")
	}

	opts.logger.Debug("Built UIA tree",
		zap.Int("count", len(controls)),
		zap.Duration("duration", time.Since(started)))

	children := make([]*TreeNode, 0, len(controls))

	for _, control := range controls {
		info := &ElementInfo{
			position:  control.bounds.Min,
			size:      image.Pt(control.bounds.Dx(), control.bounds.Dy()),
			title:     control.name,
			role:      control.role,
			isEnabled: true,
			clickable: true,
		}

		children = append(children, &TreeNode{
			element: &Element{info: info},
			info:    info,
		})
	}

	rootNode.children = children

	return rootNode, nil
}

// ProcessClickableNodes returns the clickable nodes in the tree.
func ProcessClickableNodes(root *TreeNode, _ config.HintsConfig) []*TreeNode {
	if root == nil {
		return nil
	}

	return root.FindClickableElements(nil, nil, false)
}

// ReleaseTree is a no-op: Windows nodes hold no live COM references.
func ReleaseTree(_ *TreeNode) {}

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
