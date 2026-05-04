package accessibility

import (
	"fmt"
	"image"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// InfraAXClient implements AXClient using the infrastructure layer.
type InfraAXClient struct {
	logger         *zap.Logger
	cache          *InfoCache
	configProvider config.Provider
}

// NewInfraAXClient creates a new infrastructure-based AXClient.
// If cache is nil, a default InfoCache is created automatically.
func NewInfraAXClient(
	logger *zap.Logger,
	cache *InfoCache,
	configProvider config.Provider,
) *InfraAXClient {
	if logger == nil {
		logger = zap.NewNop()
	}

	if cache == nil {
		cache = NewInfoCache(logger)
	}

	return &InfraAXClient{
		logger:         logger,
		cache:          cache,
		configProvider: configProvider,
	}
}

// Cache returns the InfoCache used by this client.
func (c *InfraAXClient) Cache() *InfoCache {
	return c.cache
}

// FrontmostWindow returns the frontmost window.
func (c *InfraAXClient) FrontmostWindow() (AXWindow, error) {
	window := FrontmostWindow()
	if window == nil {
		return nil, derrors.New(derrors.CodeAccessibilityFailed, "failed to get frontmost window")
	}

	return &InfraWindow{element: window}, nil
}

// FocusedApplication returns the focused application.
func (c *InfraAXClient) FocusedApplication() (AXApp, error) {
	app := FocusedApplication()
	if app == nil {
		return nil, derrors.New(derrors.CodeAccessibilityFailed, "failed to get focused app")
	}

	return &InfraApp{element: app}, nil
}

// ClickableNodes returns clickable nodes for the given root element.
func (c *InfraAXClient) ClickableNodes(
	root AXElement,
	includeOffscreen bool,
	roles []string,
) ([]AXNode, error) {
	var element *Element

	switch elementType := root.(type) {
	case *InfraWindow:
		element = elementType.element
	case *InfraApp:
		element = elementType.element
	default:
		return nil, derrors.New(derrors.CodeInvalidInput, "invalid element type")
	}

	if element == nil {
		return nil, derrors.New(derrors.CodeInvalidInput, "element is nil")
	}

	opts := DefaultTreeOptions(c.logger)
	opts.SetCache(c.cache)
	opts.SetIncludeOutOfBounds(includeOffscreen)

	// Enable strict filtering for Chromium/Electron apps which have noisy DOM trees
	bundleID := element.BundleIdentifier()
	if isLikelyChromiumOrElectron(bundleID) ||
		isUserConfiguredChromiumElectron(bundleID, c.configProvider) {
		opts.SetStrictFiltering(true)
		opts.SetIncludeOutOfBounds(false) // strict filtering requires bound checks to be active
	}

	if cfg := currentConfig(c.configProvider); cfg != nil {
		opts.SetMaxDepth(cfg.Hints.MaxDepth)
		opts.SetParallelThreshold(cfg.Hints.ParallelThreshold)
	}

	tree, treeErr := BuildTree(element, opts)
	if treeErr != nil {
		return nil, derrors.Wrap(
			treeErr,
			derrors.CodeAccessibilityFailed,
			"failed to build accessibility tree",
		)
	}

	var allowedRoles map[string]struct{}
	if len(roles) > 0 {
		allowedRoles = make(map[string]struct{}, len(roles))
		for _, role := range roles {
			allowedRoles[role] = struct{}{}
		}
	}

	clickableNodes := tree.FindClickableElements(allowedRoles, c.cache, c.configProvider)

	// Release tree nodes that are not part of the result to avoid
	// leaking CFRetain'd AXUIElementRefs from getChildren/getVisibleRows.
	releaseTreeExcept(tree, clickableNodes)

	clickableNodesResult := make([]AXNode, len(clickableNodes))

	for i, node := range clickableNodes {
		clickableNodesResult[i] = &InfraNode{
			node:           node,
			cache:          c.cache,
			configProvider: c.configProvider,
		}
	}

	return clickableNodesResult, nil
}

// ApplicationByBundleID returns the application with the given bundle ID.
func (c *InfraAXClient) ApplicationByBundleID(bundleID string) (AXApp, error) {
	app := ApplicationByBundleID(bundleID)
	if app == nil {
		return nil, derrors.New(derrors.CodeAccessibilityFailed, "application not found")
	}

	return &InfraApp{element: app}, nil
}

// MenuBarClickableElements returns clickable elements in the menu bar.
func (c *InfraAXClient) MenuBarClickableElements() ([]AXNode, error) {
	nodes, nodesErr := MenuBarClickableElements(c.logger, c.cache, c.configProvider)
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
			cache:          c.cache,
			configProvider: c.configProvider,
		}
	}

	return nodesResult, nil
}

// ClickableElementsFromBundleID returns clickable elements for the application with the given bundle ID.
func (c *InfraAXClient) ClickableElementsFromBundleID(
	bundleID string,
	roles []string,
	strictFiltering bool,
) ([]AXNode, error) {
	nodes, nodesErr := ClickableElementsFromBundleID(
		bundleID,
		roles,
		c.logger,
		c.cache,
		c.configProvider,
		strictFiltering,
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
			cache:          c.cache,
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

// ClearCache removes all entries from the element info cache.
func (c *InfraAXClient) ClearCache() {
	c.cache.Clear()
}

// Wrappers

// InfraWindow wraps an Window.
type InfraWindow struct {
	element *Element
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
	cache          *InfoCache
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

	return n.node.Info().RoleDescription()
}

// IsClickable returns true if the node is clickable.
func (n *InfraNode) IsClickable() bool {
	if n.node == nil || n.node.Element() == nil {
		return false
	}

	return n.node.Element().IsClickable(n.node.Info(), nil, n.cache, n.configProvider)
}

// Release releases the underlying AXUIElementRef held by this node.
func (n *InfraNode) Release() {
	if n.node != nil && n.node.Element() != nil {
		n.node.Element().Release()
	}
}

// isLikelyChromiumOrElectron returns true if the bundle ID matches known Chromium/Electron apps.
// This is duplicated from electron package to avoid import cycle.
func isLikelyChromiumOrElectron(bundleID string) bool {
	if bundleID == "" {
		return false
	}

	for _, b := range config.KnownChromiumBundles {
		if strings.EqualFold(b, bundleID) {
			return true
		}
	}

	for _, b := range config.KnownElectronBundles {
		if strings.EqualFold(b, bundleID) {
			return true
		}
	}

	return false
}

// isUserConfiguredChromiumElectron checks if the bundle ID matches user-configured
// additional Chromium/Electron bundles from config. Supports exact matches and
// wildcard patterns (ending with *).
func isUserConfiguredChromiumElectron(bundleID string, configProvider config.Provider) bool {
	if bundleID == "" || configProvider == nil {
		return false
	}

	cfg := configProvider.Get()
	if cfg == nil {
		return false
	}

	lower := strings.ToLower(bundleID)

	additionalChromium := cfg.Hints.AdditionalAXSupport.AdditionalChromiumBundles
	if config.MatchesAdditionalBundle(lower, additionalChromium) {
		return true
	}

	additionallyElectron := cfg.Hints.AdditionalAXSupport.AdditionalElectronBundles

	return config.MatchesAdditionalBundle(lower, additionallyElectron)
}
