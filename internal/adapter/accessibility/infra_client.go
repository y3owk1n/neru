package accessibility

import (
	"fmt"
	"image"

	"github.com/y3owk1n/neru/internal/domain/action"
	derrors "github.com/y3owk1n/neru/internal/errors"
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
		return nil, derrors.New(derrors.CodeAccessibilityFailed, "failed to get frontmost window")
	}

	return &infraWindow{element: window}, nil
}

// GetFocusedApplication returns the focused application.
func (c *InfraAXClient) GetFocusedApplication() (AXApp, error) {
	app := infra.GetFocusedApplication()
	if app == nil {
		return nil, derrors.New(derrors.CodeAccessibilityFailed, "failed to get focused app")
	}

	return &infraApp{element: app}, nil
}

// GetClickableNodes returns clickable nodes for the given root element.
func (c *InfraAXClient) GetClickableNodes(root AXElement, includeOffscreen bool) ([]AXNode, error) {
	var element *infra.Element

	switch elementType := root.(type) {
	case *infraWindow:
		element = elementType.element
	case *infraApp:
		element = elementType.element
	default:
		return nil, derrors.New(derrors.CodeInvalidInput, "invalid element type")
	}

	if element == nil {
		return nil, derrors.New(derrors.CodeInvalidInput, "element is nil")
	}

	opts := infra.DefaultTreeOptions()
	opts.IncludeOutOfBounds = includeOffscreen

	tree, treeErr := infra.BuildTree(element, opts)
	if treeErr != nil {
		return nil, derrors.Wrap(
			treeErr,
			derrors.CodeAccessibilityFailed,
			"failed to build accessibility tree",
		)
	}

	clickableNodes := tree.FindClickableElements()

	clickableNodesResult := make([]AXNode, len(clickableNodes))
	for i, node := range clickableNodes {
		clickableNodesResult[i] = &infraNode{node: node}
	}

	return clickableNodesResult, nil
}

// GetApplicationByBundleID returns the application with the given bundle ID.
func (c *InfraAXClient) GetApplicationByBundleID(bundleID string) (AXApp, error) {
	app := infra.GetApplicationByBundleID(bundleID)
	if app == nil {
		return nil, derrors.New(derrors.CodeAccessibilityFailed, "application not found")
	}

	return &infraApp{element: app}, nil
}

// GetMenuBarClickableElements returns clickable elements in the menu bar.
func (c *InfraAXClient) GetMenuBarClickableElements() ([]AXNode, error) {
	nodes, nodesErr := infra.GetMenuBarClickableElements()
	if nodesErr != nil {
		return nil, derrors.Wrap(
			nodesErr,
			derrors.CodeAccessibilityFailed,
			"failed to get menu bar elements",
		)
	}

	nodesResult := make([]AXNode, len(nodes))
	for index, node := range nodes {
		nodesResult[index] = &infraNode{node: node}
	}

	return nodesResult, nil
}

// GetClickableElementsFromBundleID returns clickable elements for the application with the given bundle ID.
func (c *InfraAXClient) GetClickableElementsFromBundleID(bundleID string) ([]AXNode, error) {
	nodes, nodesErr := infra.GetClickableElementsFromBundleID(bundleID)
	if nodesErr != nil {
		return nil, derrors.Wrap(
			nodesErr,
			derrors.CodeAccessibilityFailed,
			"failed to get elements from bundle ID",
		)
	}

	nodesResult := make([]AXNode, len(nodes))
	for index, node := range nodes {
		nodesResult[index] = &infraNode{node: node}
	}

	return nodesResult, nil
}

// GetActiveScreenBounds returns the bounds of the active screen.
func (c *InfraAXClient) GetActiveScreenBounds() image.Rectangle {
	return bridge.GetActiveScreenBounds()
}

// PerformAction performs the specified action at the given point.
func (c *InfraAXClient) PerformAction(
	actionType action.Type,
	point image.Point,
	restoreCursor bool,
) error {
	var performActionErr error

	switch actionType {
	case action.TypeLeftClick:
		performActionErr = infra.LeftClickAtPoint(point, restoreCursor)
	case action.TypeRightClick:
		performActionErr = infra.RightClickAtPoint(point, restoreCursor)
	case action.TypeMiddleClick:
		performActionErr = infra.MiddleClickAtPoint(point, restoreCursor)
	case action.TypeMouseDown:
		performActionErr = infra.LeftMouseDownAtPoint(point)
	case action.TypeMouseUp:
		performActionErr = infra.LeftMouseUpAtPoint(point)
	case action.TypeMoveMouse:
		infra.MoveMouseToPoint(point)

		return nil
	case action.TypeScroll:
		// Scroll actions are handled separately via the Scroll method
		return derrors.Newf(
			derrors.CodeInvalidInput,
			"scroll actions should use Scroll method: %s",
			actionType,
		)
	default:
		return derrors.Newf(derrors.CodeInvalidInput, "unsupported action type: %s", actionType)
	}

	if performActionErr != nil {
		return derrors.Wrap(performActionErr, derrors.CodeActionFailed, "failed to perform action")
	}

	return nil
}

// Scroll performs a scroll action.
func (c *InfraAXClient) Scroll(deltaX, deltaY int) error {
	scrollErr := infra.ScrollAtCursor(deltaX, deltaY)
	if scrollErr != nil {
		return derrors.Wrap(scrollErr, derrors.CodeActionFailed, "failed to scroll")
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
	element *infra.Element
}

func (w *infraWindow) Release() {
	if w.element != nil {
		w.element.Release()
	}
}

type infraApp struct {
	element *infra.Element
}

func (a *infraApp) Release() {
	if a.element != nil {
		a.element.Release()
	}
}

func (a *infraApp) GetBundleIdentifier() string {
	if a.element != nil {
		return a.element.GetBundleIdentifier()
	}

	return ""
}

func (a *infraApp) GetInfo() (*AXAppInfo, error) {
	if a.element == nil {
		return nil, derrors.New(derrors.CodeInvalidInput, "element is nil")
	}

	info, infoErr := a.element.GetInfo()
	if infoErr != nil {
		return nil, infoErr
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
