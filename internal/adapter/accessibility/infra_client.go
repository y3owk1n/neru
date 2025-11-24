package accessibility

import (
	"fmt"
	"image"

	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/errors"
	infra "github.com/y3owk1n/neru/internal/infra/accessibility"
	"github.com/y3owk1n/neru/internal/infra/bridge"
)

// InfraAXClient implements AXClient using the infrastructure layer.
type InfraAXClient struct{}

// NewInfraAXClient creates a new infrastructure-based AXClient.
func NewInfraAXClient() *InfraAXClient {
	return &InfraAXClient{}
}

// GetFrontmostWindow returns the frontmost window.
func (c *InfraAXClient) GetFrontmostWindow() (AXWindow, error) {
	window := infra.GetFrontmostWindow()
	if window == nil {
		return nil, errors.New(errors.CodeAccessibilityFailed, "failed to get frontmost window")
	}
	return &infraWindow{elem: window}, nil
}

// GetFocusedApplication returns the focused application.
func (c *InfraAXClient) GetFocusedApplication() (AXApp, error) {
	app := infra.GetFocusedApplication()
	if app == nil {
		return nil, errors.New(errors.CodeAccessibilityFailed, "failed to get focused app")
	}
	return &infraApp{elem: app}, nil
}

// GetClickableNodes returns clickable nodes for the given root element.
func (c *InfraAXClient) GetClickableNodes(root AXElement, includeOffscreen bool) ([]AXNode, error) {
	var elem *infra.Element

	switch v := root.(type) {
	case *infraWindow:
		elem = v.elem
	case *infraApp:
		elem = v.elem
	default:
		return nil, errors.New(errors.CodeInvalidInput, "invalid element type")
	}

	if elem == nil {
		return nil, errors.New(errors.CodeInvalidInput, "element is nil")
	}

	opts := infra.DefaultTreeOptions()
	opts.IncludeOutOfBounds = includeOffscreen

	tree, err := infra.BuildTree(elem, opts)
	if err != nil {
		return nil, errors.Wrap(
			err,
			errors.CodeAccessibilityFailed,
			"failed to build accessibility tree",
		)
	}

	clickableNodes := tree.FindClickableElements()
	result := make([]AXNode, len(clickableNodes))
	for i, node := range clickableNodes {
		result[i] = &infraNode{node: node}
	}

	return result, nil
}

// GetApplicationByBundleID returns the application with the given bundle ID.
func (c *InfraAXClient) GetApplicationByBundleID(bundleID string) (AXApp, error) {
	app := infra.GetApplicationByBundleID(bundleID)
	if app == nil {
		return nil, errors.New(errors.CodeAccessibilityFailed, "application not found")
	}
	return &infraApp{elem: app}, nil
}

// GetMenuBarClickableElements returns clickable elements in the menu bar.
func (c *InfraAXClient) GetMenuBarClickableElements() ([]AXNode, error) {
	nodes, err := infra.GetMenuBarClickableElements()
	if err != nil {
		return nil, errors.Wrap(
			err,
			errors.CodeAccessibilityFailed,
			"failed to get menu bar elements",
		)
	}

	result := make([]AXNode, len(nodes))
	for i, node := range nodes {
		result[i] = &infraNode{node: node}
	}
	return result, nil
}

// GetClickableElementsFromBundleID returns clickable elements for the application with the given bundle ID.
func (c *InfraAXClient) GetClickableElementsFromBundleID(bundleID string) ([]AXNode, error) {
	nodes, err := infra.GetClickableElementsFromBundleID(bundleID)
	if err != nil {
		return nil, errors.Wrap(
			err,
			errors.CodeAccessibilityFailed,
			"failed to get elements from bundle ID",
		)
	}

	result := make([]AXNode, len(nodes))
	for i, node := range nodes {
		result[i] = &infraNode{node: node}
	}
	return result, nil
}

// GetActiveScreenBounds returns the bounds of the active screen.
func (c *InfraAXClient) GetActiveScreenBounds() image.Rectangle {
	return bridge.GetActiveScreenBounds()
}

// PerformAction performs the specified action at the given point.
func (c *InfraAXClient) PerformAction(
	actionType action.Type,
	p image.Point,
	restoreCursor bool,
) error {
	var err error
	switch actionType {
	case action.TypeLeftClick:
		err = infra.LeftClickAtPoint(p, restoreCursor)
	case action.TypeRightClick:
		err = infra.RightClickAtPoint(p, restoreCursor)
	case action.TypeMiddleClick:
		err = infra.MiddleClickAtPoint(p, restoreCursor)
	case action.TypeMouseDown:
		err = infra.LeftMouseDownAtPoint(p)
	case action.TypeMouseUp:
		err = infra.LeftMouseUpAtPoint(p)
	case action.TypeMoveMouse:
		infra.MoveMouseToPoint(p)
		return nil
	default:
		return errors.Newf(errors.CodeInvalidInput, "unsupported action type: %s", actionType)
	}

	if err != nil {
		return errors.Wrap(err, errors.CodeActionFailed, "failed to perform action")
	}
	return nil
}

// Scroll performs a scroll action.
func (c *InfraAXClient) Scroll(deltaX, deltaY int) error {
	err := infra.ScrollAtCursor(deltaX, deltaY)
	if err != nil {
		return errors.Wrap(err, errors.CodeActionFailed, "failed to scroll")
	}
	return nil
}

// MoveMouse moves the mouse to the specified point.
func (c *InfraAXClient) MoveMouse(p image.Point) {
	infra.MoveMouseToPoint(p)
}

// GetCursorPosition returns the current cursor position.
func (c *InfraAXClient) GetCursorPosition() image.Point {
	return infra.GetCurrentCursorPosition()
}

// CheckPermissions checks if accessibility permissions are granted.
func (c *InfraAXClient) CheckPermissions() bool {
	return infra.CheckAccessibilityPermissions()
}

// SetClickableRoles sets the roles that are considered clickable.
func (c *InfraAXClient) SetClickableRoles(roles []string) {
	infra.SetClickableRoles(roles)
}

// GetClickableRoles returns the roles that are considered clickable.
func (c *InfraAXClient) GetClickableRoles() []string {
	return infra.GetClickableRoles()
}

// IsMissionControlActive checks if Mission Control is currently active.
func (c *InfraAXClient) IsMissionControlActive() bool {
	return infra.IsMissionControlActive()
}

// Wrappers

type infraWindow struct {
	elem *infra.Element
}

func (w *infraWindow) Release() {
	if w.elem != nil {
		w.elem.Release()
	}
}

type infraApp struct {
	elem *infra.Element
}

func (a *infraApp) Release() {
	if a.elem != nil {
		a.elem.Release()
	}
}

func (a *infraApp) GetBundleIdentifier() string {
	if a.elem != nil {
		return a.elem.GetBundleIdentifier()
	}
	return ""
}

func (a *infraApp) GetInfo() (*AXAppInfo, error) {
	if a.elem == nil {
		return nil, errors.New(errors.CodeInvalidInput, "element is nil")
	}
	info, err := a.elem.GetInfo()
	if err != nil {
		return nil, err
	}
	return &AXAppInfo{
		Role:  info.Role,
		Title: info.Title,
	}, nil
}

type infraNode struct {
	node *infra.TreeNode
}

func (n *infraNode) GetID() string {
	if n.node == nil {
		return ""
	}
	return fmt.Sprintf("elem_%p", n.node.Element)
}

func (n *infraNode) GetBounds() image.Rectangle {
	if n.node == nil || n.node.Info == nil {
		return image.Rectangle{}
	}
	info := n.node.Info
	return image.Rect(
		info.Position.X,
		info.Position.Y,
		info.Position.X+info.Size.X,
		info.Position.Y+info.Size.Y,
	)
}

func (n *infraNode) GetRole() string {
	if n.node == nil || n.node.Info == nil {
		return ""
	}
	return n.node.Info.Role
}

func (n *infraNode) GetTitle() string {
	if n.node == nil || n.node.Info == nil {
		return ""
	}
	return n.node.Info.Title
}

func (n *infraNode) GetDescription() string {
	if n.node == nil || n.node.Info == nil {
		return ""
	}
	return n.node.Info.RoleDescription
}

func (n *infraNode) IsClickable() bool {
	if n.node == nil || n.node.Element == nil {
		return false
	}
	return n.node.Element.IsClickable(n.node.Info)
}
