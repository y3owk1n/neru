//go:build windows

package accessibility

import (
	"context"
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/element"
)

// TreeNode represents a node in the accessibility element hierarchy. On Windows
// the tree is shallow: a root window node whose children are the flat list of
// clickable controls discovered via UI Automation. Each node owns a unique
// *Element so callers can use the pointer identity as a stable element ID.
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
			if windowsRoleMatchesFilter(node.info.role, keptRoles) {
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
}

// DefaultTreeOptions returns the default tree options.
func DefaultTreeOptions(_ *zap.Logger) TreeOptions { return TreeOptions{} }

// SetCache is a no-op on Windows (no native cache request).
func (o *TreeOptions) SetCache(_ any) {}

// SetMaxDepth records the maximum tree depth.
func (o *TreeOptions) SetMaxDepth(depth int) { o.MaxDepth = depth }

// SetBundleID is a no-op on Windows.
func (o *TreeOptions) SetBundleID(_ string) {}

// SetConfigProvider is a no-op on Windows.
func (o *TreeOptions) SetConfigProvider(_ config.Provider) {}

// SetFilterFunc is a no-op on Windows.
func (o *TreeOptions) SetFilterFunc(_ func(*ElementInfo) bool) {}

// BuildTree enumerates the clickable controls under the given window element
// via UI Automation and returns a root node with one child per control. For
// non-window elements (no HWND) it returns an empty root.
func BuildTree(_ context.Context, root *Element, _ TreeOptions) (*TreeNode, error) {
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

	controls := enumerateClickableElements(root.hwnd)

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

// windowsUIAToAXRole maps legacy UIA control-type names that may still appear
// in user configs to the AX-style roles assigned during enumeration.
var windowsUIAToAXRole = map[string]string{
	"Button":      string(element.RoleButton),
	"CheckBox":    string(element.RoleCheckBox),
	"RadioButton": string(element.RoleRadioButton),
	"Hyperlink":   string(element.RoleLink),
	"ComboBox":    string(element.RoleComboBox),
	"Edit":        string(element.RoleTextField),
	"Slider":      string(element.RoleSlider),
	"TabItem":     string(element.RoleTabButton),
	"MenuItem":    string(element.RoleMenuItem),
	"DataItem":    string(element.RoleCell),
	"ListItem":    string(element.RoleCell),
	"TreeItem":    string(element.RoleRow),
	"Spinner":     string(element.RoleIncrementor),
	"SplitButton": string(element.RoleButton),
}

// windowsRoleMatchesFilter reports whether elementRole satisfies keptRoles.
// An empty keptRoles set accepts every clickable element. Configured roles
// may use either AX-style names or legacy UIA control-type names.
func windowsRoleMatchesFilter(elementRole string, keptRoles map[string]struct{}) bool {
	if len(keptRoles) == 0 {
		return true
	}

	if _, ok := keptRoles[elementRole]; ok {
		return true
	}

	for configRole := range keptRoles {
		if axRole, ok := windowsUIAToAXRole[configRole]; ok && axRole == elementRole {
			return true
		}
	}

	return false
}
