package accessibility

import (
	"context"
	"fmt"
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// InfraAXClient implements AXClient using the infrastructure layer.
type InfraAXClient struct {
	logger         *zap.Logger
	configProvider config.Provider
}

// NewInfraAXClient creates a new infrastructure-based AXClient.
func NewInfraAXClient(
	logger *zap.Logger,
	configProvider config.Provider,
) *InfraAXClient {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &InfraAXClient{
		logger:         logger,
		configProvider: configProvider,
	}
}

// FrontmostWindow returns the frontmost window.
func (c *InfraAXClient) FrontmostWindow(_ context.Context) (AXWindow, error) {
	window := FrontmostWindow()
	if window == nil {
		return nil, derrors.New(derrors.CodeAccessibilityFailed, "failed to get frontmost window")
	}

	return &InfraWindow{element: window}, nil
}

// AllWindows returns all windows of the focused application.
func (c *InfraAXClient) AllWindows(_ context.Context) ([]AXWindow, error) {
	windows, err := AllWindows()
	if err != nil {
		return nil, err
	}

	result := make([]AXWindow, len(windows))
	for i, w := range windows {
		result[i] = &InfraWindow{element: w}
	}

	return result, nil
}

// FrontmostAndPopoverWindows returns the frontmost window plus popovers.
func (c *InfraAXClient) FrontmostAndPopoverWindows(_ context.Context) ([]AXWindow, error) {
	windows, err := FrontmostAndPopoverWindows()
	if err != nil {
		return nil, err
	}

	result := make([]AXWindow, len(windows))
	for i, w := range windows {
		result[i] = &InfraWindow{element: w}
	}

	return result, nil
}

// FocusedApplication returns the focused application.
func (c *InfraAXClient) FocusedApplication(_ context.Context) (AXApp, error) {
	app := FocusedApplication()
	if app == nil {
		return nil, derrors.New(derrors.CodeAccessibilityFailed, "failed to get focused app")
	}

	return &InfraApp{element: app}, nil
}

// ClickableNodes returns clickable nodes for the given root element.
// If maxDepth is > 0, it overrides the configured tree depth.
func (c *InfraAXClient) ClickableNodes(
	ctx context.Context,
	root AXElement,
	roles []string,
	maxDepth int,
) ([]AXNode, error) {
	element := c.extractElement(root)

	if element == nil {
		return nil, derrors.New(derrors.CodeInvalidInput, "element is nil")
	}

	opts, allowedRoles, ignoreClickableCheck := c.buildClickableOpts(element, roles, maxDepth)

	tree, treeErr := BuildTree(ctx, element, opts)
	if treeErr != nil {
		return nil, derrors.Wrap(
			treeErr,
			derrors.CodeAccessibilityFailed,
			"failed to build accessibility tree",
		)
	}

	clickableNodes := tree.FindClickableElements(
		allowedRoles,
		c.configProvider,
		ignoreClickableCheck,
	)

	// Release tree nodes that are not part of the result to avoid
	// leaking CFRetain'd AXUIElementRefs from getChildren/getVisibleRows.
	releaseTreeExcept(tree, clickableNodes)

	clickableNodesResult := make([]AXNode, len(clickableNodes))

	for i, node := range clickableNodes {
		clickableNodesResult[i] = &InfraNode{
			node:           node,
			clickable:      true,
			configProvider: c.configProvider,
		}
	}

	return clickableNodesResult, nil
}

// ApplicationByBundleID returns the application with the given bundle ID.
func (c *InfraAXClient) ApplicationByBundleID(_ context.Context, bundleID string) (AXApp, error) {
	app := ApplicationByBundleID(bundleID)
	if app == nil {
		return nil, derrors.New(derrors.CodeAccessibilityFailed, "application not found")
	}

	return &InfraApp{element: app}, nil
}

// MenuBarClickableElements returns clickable elements in the menu bar.
// If maxDepth is > 0, it overrides the configured tree depth.
func (c *InfraAXClient) MenuBarClickableElements(
	ctx context.Context,
	maxDepth int,
) ([]AXNode, error) {
	nodes, nodesErr := MenuBarClickableElements(
		ctx,
		c.logger,
		c.configProvider,
		maxDepth,
	)
	if nodesErr != nil {
		return nil, derrors.Wrap(
			nodesErr,
			derrors.CodeAccessibilityFailed,
			"failed to get menu bar elements",
		)
	}

	nodesResult := make([]AXNode, len(nodes))
	for index, node := range nodes {
		nodesResult[index] = &InfraNode{
			node:           node,
			clickable:      true,
			configProvider: c.configProvider,
		}
	}

	return nodesResult, nil
}

// ClickableElementsFromBundleID returns clickable elements for the application with the given bundle ID.
// If maxDepth is > 0, it overrides the configured tree depth for flat supplementary sources.
func (c *InfraAXClient) ClickableElementsFromBundleID(
	ctx context.Context,
	bundleID string,
	roles []string,
	maxDepth int,
) ([]AXNode, error) {
	nodes, nodesErr := ClickableElementsFromBundleID(
		ctx,
		bundleID,
		roles,
		c.logger,
		c.configProvider,
		maxDepth,
	)
	if nodesErr != nil {
		return nil, derrors.Wrap(
			nodesErr,
			derrors.CodeAccessibilityFailed,
			"failed to get elements from bundle ID",
		)
	}

	nodesResult := make([]AXNode, len(nodes))
	for index, node := range nodes {
		nodesResult[index] = &InfraNode{
			node:           node,
			clickable:      true,
			configProvider: c.configProvider,
		}
	}

	return nodesResult, nil
}

// ActiveScreenBounds returns the bounds of the active screen.
func (c *InfraAXClient) ActiveScreenBounds() image.Rectangle {
	return platformActiveScreenBounds()
}

// PerformAction performs the specified action at the given point.
func (c *InfraAXClient) PerformAction(
	actionType action.Type,
	point image.Point,
	restoreCursor bool,
	modifiers action.Modifiers,
) error {
	var performActionErr error

	switch actionType {
	case action.TypeLeftClick:
		performActionErr = LeftClickAtPoint(point, restoreCursor, modifiers)
	case action.TypeRightClick:
		EnsureMouseUp()

		performActionErr = RightClickAtPoint(point, restoreCursor, modifiers)
	case action.TypeMiddleClick:
		EnsureMouseUp()

		performActionErr = MiddleClickAtPoint(point, restoreCursor, modifiers)
	case action.TypeMouseDown:
		performActionErr = LeftMouseDownAtPoint(point, modifiers)
	case action.TypeMouseUp:
		performActionErr = LeftMouseUpAtPoint(point, modifiers)
	case action.TypeMoveMouse, action.TypeMoveMouseRelative:
		MoveMouseToPoint(point, false)

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
	EnsureMouseUp()

	scrollErr := ScrollAtCursor(deltaX, deltaY)
	if scrollErr != nil {
		return derrors.Wrap(scrollErr, derrors.CodeActionFailed, "failed to scroll")
	}

	return nil
}

// MoveMouse moves the mouse to the specified point.
func (c *InfraAXClient) MoveMouse(p image.Point, bypassSmooth bool) {
	MoveMouseToPoint(p, bypassSmooth)
}

// CursorPosition returns the current cursor position.
func (c *InfraAXClient) CursorPosition() image.Point {
	return CurrentCursorPosition()
}

// CheckPermissions checks if accessibility permissions are granted.
func (c *InfraAXClient) CheckPermissions() bool {
	return CheckAccessibilityPermissions()
}

// SetClickableRoles sets the roles that are considered clickable.
func (c *InfraAXClient) SetClickableRoles(roles []string) {
	SetClickableRoles(roles, c.logger)
}

// ClickableRoles returns the roles that are considered clickable.
func (c *InfraAXClient) ClickableRoles() []string {
	return ClickableRoles()
}

// IsMissionControlActive checks if Mission Control is currently active.
func (c *InfraAXClient) IsMissionControlActive() bool {
	return IsMissionControlActive()
}

// extractElement returns the raw *Element from an AXElement wrapper.
func (c *InfraAXClient) extractElement(root AXElement) *Element {
	switch elementType := root.(type) {
	case *InfraWindow:
		return elementType.element
	case *InfraApp:
		return elementType.element
	default:
		return nil
	}
}

// buildClickableOpts constructs tree options, allowed roles, and the
// ignore-clickable flag for the given element and role list.
func (c *InfraAXClient) buildClickableOpts(
	element *Element,
	roles []string,
	maxDepth int,
) (TreeOptions, map[string]struct{}, bool) {
	opts := DefaultTreeOptions(c.logger)
	opts.SetConfigProvider(c.configProvider)

	if cfg := currentConfig(c.configProvider); cfg != nil {
		depth := cfg.Hints.MaxDepth
		if maxDepth > 0 {
			depth = maxDepth
		}

		opts.SetMaxDepth(depth)
	}

	var allowedRoles map[string]struct{}
	if len(roles) > 0 {
		allowedRoles = make(map[string]struct{}, len(roles))
		for _, role := range roles {
			allowedRoles[role] = struct{}{}
		}
	}

	ignoreClickableCheck := false
	if cfg := currentConfig(c.configProvider); cfg != nil {
		ignoreClickableCheck = cfg.ShouldIgnoreClickableCheckForApp(element.BundleIdentifier())
	}

	return opts, allowedRoles, ignoreClickableCheck
}

// Wrappers

// InfraWindow wraps an Window.
type InfraWindow struct {
	element *Element
}

// Role returns the window role (e.g., "AXWindow", "AXPopover").
func (w *InfraWindow) Role() string {
	if w.element != nil {
		info, err := w.element.Info()
		if err == nil && info != nil {
			return info.Role()
		}
	}

	return ""
}

// Release releases the Window.
func (w *InfraWindow) Release() {
	if w.element != nil {
		w.element.Release()
	}
}

// InfraApp wraps an Element.
type InfraApp struct {
	element *Element
}

// Release releases the Element.
func (a *InfraApp) Release() {
	if a.element != nil {
		a.element.Release()
	}
}

// BundleIdentifier returns the bundle identifier.
func (a *InfraApp) BundleIdentifier() string {
	if a.element != nil {
		return a.element.BundleIdentifier()
	}

	return ""
}

// Info returns the app info.
func (a *InfraApp) Info() (*AXAppInfo, error) {
	if a.element == nil {
		return nil, derrors.New(derrors.CodeInvalidInput, "element is nil")
	}

	info, infoErr := a.element.Info()
	if infoErr != nil {
		return nil, infoErr
	}

	return &AXAppInfo{
		Role:  info.Role(),
		Title: info.Title(),
	}, nil
}

// InfraNode wraps an TreeNode.
type InfraNode struct {
	node           *TreeNode
	clickable      bool
	configProvider config.Provider
}

// ID returns the node ID.
func (n *InfraNode) ID() string {
	if n.node == nil {
		return ""
	}

	return fmt.Sprintf("elem_%p", n.node.Element())
}

// Bounds returns the node bounds.
func (n *InfraNode) Bounds() image.Rectangle {
	if n.node == nil || n.node.Info() == nil {
		return image.Rectangle{}
	}

	info := n.node.Info()
	pos := info.Position()
	size := info.Size()

	return image.Rect(
		pos.X,
		pos.Y,
		pos.X+size.X,
		pos.Y+size.Y,
	)
}

// Role returns the node role.
func (n *InfraNode) Role() string {
	if n.node == nil || n.node.Info() == nil {
		return ""
	}

	return n.node.Info().Role()
}

// Title returns the node title.
func (n *InfraNode) Title() string {
	if n.node == nil || n.node.Info() == nil {
		return ""
	}

	return n.node.Info().Title()
}

// Description returns the node description.
func (n *InfraNode) Description() string {
	if n.node == nil || n.node.Info() == nil {
		return ""
	}

	return n.node.Info().Description()
}

// Value returns the node value.
func (n *InfraNode) Value() string {
	if n.node == nil || n.node.Info() == nil {
		return ""
	}

	return n.node.Info().Value()
}

// SearchText returns additional text collected from the node subtree.
func (n *InfraNode) SearchText() string {
	if n.node == nil || n.node.Info() == nil {
		return ""
	}

	return n.node.Info().SearchText()
}

// IsClickable returns true if the node is clickable.
func (n *InfraNode) IsClickable() bool {
	if n.clickable {
		return true
	}

	if n.node == nil || n.node.Element() == nil {
		return false
	}

	return n.node.Element().IsClickable(n.node.Info(), nil, n.configProvider, false)
}

// Release releases the underlying AXUIElementRef held by this node.
func (n *InfraNode) Release() {
	if n.node != nil && n.node.Element() != nil {
		n.node.Element().Release()
	}
}
