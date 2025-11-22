package accessibility

import (
	"context"
	"fmt"
	"image"

	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/errors"
	infra "github.com/y3owk1n/neru/internal/infra/accessibility"
	"github.com/y3owk1n/neru/internal/infra/bridge"
	"go.uber.org/zap"
)

// Adapter implements ports.AccessibilityPort by wrapping the CGo bridge.
// It converts between domain models and infrastructure types.
type Adapter struct {
	logger          *zap.Logger
	excludedBundles map[string]bool
	clickableRoles  []string
}

// NewAdapter creates a new accessibility adapter.
func NewAdapter(logger *zap.Logger, excludedBundles []string, clickableRoles []string) *Adapter {
	excludedMap := make(map[string]bool, len(excludedBundles))
	for _, bundle := range excludedBundles {
		excludedMap[bundle] = true
	}

	return &Adapter{
		logger:          logger,
		excludedBundles: excludedMap,
		clickableRoles:  clickableRoles,
	}
}

// GetClickableElements retrieves all clickable UI elements matching the filter.
func (a *Adapter) GetClickableElements(
	ctx context.Context,
	filter ports.ElementFilter,
) ([]*element.Element, error) {
	// Check context
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	a.logger.Debug("Getting clickable elements", zap.Any("filter", filter))

	// Get frontmost window
	window := infra.GetFrontmostWindow()
	if window == nil {
		return nil, errors.New(errors.CodeAccessibilityFailed, "failed to get frontmost window")
	}
	defer window.Release()

	// Build accessibility tree
	opts := infra.DefaultTreeOptions()
	opts.IncludeOutOfBounds = filter.IncludeOffscreen

	tree, err := infra.BuildTree(window, opts)
	if err != nil {
		return nil, errors.Wrap(
			err,
			errors.CodeAccessibilityFailed,
			"failed to build accessibility tree",
		)
	}

	// Find clickable elements
	clickableNodes := tree.FindClickableElements()

	a.logger.Debug("Found clickable nodes", zap.Int("count", len(clickableNodes)))

	// Convert to domain elements
	elements := make([]*element.Element, 0, len(clickableNodes))
	for i, node := range clickableNodes {
		// Check context periodically
		if i%100 == 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
		}

		elem, err := a.convertToDomainElement(node)
		if err != nil {
			a.logger.Warn("Failed to convert element", zap.Error(err))
			continue
		}

		// Apply filter
		if a.matchesFilter(elem, filter) {
			elements = append(elements, elem)
		}
	}

	a.logger.Info("Converted to domain elements", zap.Int("count", len(elements)))
	return elements, nil
}

// GetScrollableElements retrieves all scrollable UI elements.
func (a *Adapter) GetScrollableElements(ctx context.Context) ([]*element.Element, error) {
	// Check context
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	a.logger.Debug("Getting scrollable elements")

	// Get focused app
	focusedApp := infra.GetFocusedApplication()
	if focusedApp == nil {
		return nil, errors.New(errors.CodeAccessibilityFailed, "failed to get focused app")
	}
	defer focusedApp.Release()

	// Build accessibility tree
	opts := infra.DefaultTreeOptions()
	tree, err := infra.BuildTree(focusedApp, opts)
	if err != nil {
		return nil, errors.Wrap(
			err,
			errors.CodeAccessibilityFailed,
			"failed to build accessibility tree",
		)
	}

	// Find scrollable elements (placeholder - would need to implement in infra)
	// For now, return empty list
	_ = tree // Use tree to avoid unused variable
	a.logger.Debug("Scrollable elements not yet implemented")
	return []*element.Element{}, nil
}

// PerformAction executes an action on the specified element.
func (a *Adapter) PerformAction(
	ctx context.Context,
	elem *element.Element,
	actionType action.Type,
) error {
	// Check context
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	a.logger.Info("Performing action",
		zap.String("action", actionType.String()),
		zap.String("element_id", string(elem.ID())))

	// Get the center point of the element
	center := elem.Center()

	// Get restore cursor setting from config
	cfg := config.Global()
	restoreCursor := cfg != nil && cfg.General.RestoreCursorPosition

	// Perform the action based on type
	var err error
	switch actionType {
	case action.TypeLeftClick:
		err = infra.LeftClickAtPoint(center, restoreCursor)
	case action.TypeRightClick:
		err = infra.RightClickAtPoint(center, restoreCursor)
	case action.TypeMiddleClick:
		err = infra.MiddleClickAtPoint(center, restoreCursor)
	case action.TypeMouseDown:
		err = infra.LeftMouseDownAtPoint(center)
	case action.TypeMouseUp:
		err = infra.LeftMouseUpAtPoint(center)
	case action.TypeMoveMouse:
		infra.MoveMouseToPoint(center)
		return nil
	default:
		return errors.Newf(errors.CodeInvalidInput, "unsupported action type: %s", actionType)
	}

	if err != nil {
		return errors.Wrap(err, errors.CodeActionFailed, "failed to perform action")
	}

	return nil
}

// PerformActionAtPoint executes an action at the specified point.
func (a *Adapter) PerformActionAtPoint(
	ctx context.Context,
	actionType action.Type,
	point image.Point,
) error {
	// Check context
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	a.logger.Info("Performing action at point",
		zap.String("action", actionType.String()),
		zap.Int("x", point.X),
		zap.Int("y", point.Y))

	// Get restore cursor setting from config
	cfg := config.Global()
	restoreCursor := cfg != nil && cfg.General.RestoreCursorPosition

	// Perform the action based on type
	var err error
	switch actionType {
	case action.TypeLeftClick:
		err = infra.LeftClickAtPoint(point, restoreCursor)
	case action.TypeRightClick:
		err = infra.RightClickAtPoint(point, restoreCursor)
	case action.TypeMiddleClick:
		err = infra.MiddleClickAtPoint(point, restoreCursor)
	case action.TypeMouseDown:
		err = infra.LeftMouseDownAtPoint(point)
	case action.TypeMouseUp:
		err = infra.LeftMouseUpAtPoint(point)
	case action.TypeMoveMouse:
		infra.MoveMouseToPoint(point)
		return nil
	default:
		return errors.Newf(errors.CodeInvalidInput, "unsupported action type: %s", actionType)
	}

	if err != nil {
		return errors.Wrap(err, errors.CodeActionFailed, "failed to perform action at point")
	}

	return nil
}

// Scroll performs a scroll action at the current cursor position.
func (a *Adapter) Scroll(ctx context.Context, deltaX, deltaY int) error {
	a.logger.Debug("Performing scroll",
		zap.Int("deltaX", deltaX),
		zap.Int("deltaY", deltaY))

	// Use the infra layer to perform the actual scroll
	// The infra.ScrollAtCursor function handles the CGo bridge to macOS
	infra.ScrollAtCursor(deltaX, deltaY)

	a.logger.Debug("Scroll completed")
	return nil
}

// MoveCursorToPoint moves the mouse cursor to the specified point.
func (a *Adapter) MoveCursorToPoint(ctx context.Context, point image.Point) error {
	a.logger.Debug("Moving cursor to point",
		zap.Int("x", point.X),
		zap.Int("y", point.Y))

	infra.MoveMouseToPoint(point)
	return nil
}

// GetCursorPosition returns the current cursor position.
func (a *Adapter) GetCursorPosition(ctx context.Context) (image.Point, error) {
	pos := infra.GetCurrentCursorPosition()
	a.logger.Debug("Got cursor position",
		zap.Int("x", pos.X),
		zap.Int("y", pos.Y))
	return pos, nil
}

// GetFocusedAppBundleID returns the bundle ID of the currently focused application.
func (a *Adapter) GetFocusedAppBundleID(ctx context.Context) (string, error) {
	// Check context
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	focusedApp := infra.GetFocusedApplication()
	if focusedApp == nil {
		return "", errors.New(errors.CodeAccessibilityFailed, "failed to get focused application")
	}
	defer focusedApp.Release()

	bundleID := focusedApp.GetBundleIdentifier()
	if bundleID == "" {
		return "", errors.New(errors.CodeAccessibilityFailed, "failed to get bundle ID")
	}

	return bundleID, nil
}

// IsAppExcluded checks if the given bundle ID is in the exclusion list.
func (a *Adapter) IsAppExcluded(ctx context.Context, bundleID string) bool {
	return a.excludedBundles[bundleID]
}

// GetScreenBounds returns the bounds of the active screen.
func (a *Adapter) GetScreenBounds(ctx context.Context) (image.Rectangle, error) {
	// Check context
	select {
	case <-ctx.Done():
		return image.Rectangle{}, ctx.Err()
	default:
	}

	bounds := bridge.GetActiveScreenBounds()
	return bounds, nil
}

// CheckPermissions verifies that accessibility permissions are granted.
func (a *Adapter) CheckPermissions(ctx context.Context) error {
	// Check context
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if !infra.CheckAccessibilityPermissions() {
		return errors.New(errors.CodeAccessibilityDenied,
			"accessibility permissions not granted - please enable in System Preferences")
	}

	return nil
}

// UpdateClickableRoles updates the list of clickable roles.
func (a *Adapter) UpdateClickableRoles(roles []string) {
	a.logger.Info("Updating clickable roles", zap.Int("count", len(roles)))
	a.clickableRoles = roles
	infra.SetClickableRoles(roles)
}

// UpdateExcludedBundles updates the list of excluded bundle IDs.
func (a *Adapter) UpdateExcludedBundles(bundles []string) {
	a.logger.Info("Updating excluded bundles", zap.Int("count", len(bundles)))
	a.excludedBundles = make(map[string]bool, len(bundles))
	for _, bundle := range bundles {
		a.excludedBundles[bundle] = true
	}
}

// convertToDomainElement converts an infrastructure TreeNode to a domain Element.
func (a *Adapter) convertToDomainElement(node *infra.TreeNode) (*element.Element, error) {
	if node == nil || node.Info == nil {
		return nil, fmt.Errorf("node or node info is nil")
	}

	info := node.Info

	// Create element ID from pointer address (unique identifier)
	elemID := element.ID(fmt.Sprintf("elem_%p", node.Element))

	// Convert bounds from Position and Size
	bounds := image.Rect(
		info.Position.X,
		info.Position.Y,
		info.Position.X+info.Size.X,
		info.Position.Y+info.Size.Y,
	)

	// Convert role
	role := element.Role(info.Role)

	// Determine if clickable
	isClickable := node.Element != nil && node.Element.IsClickable(node.Info)

	// Create element with options
	elem, err := element.NewElement(
		elemID,
		bounds,
		role,
		element.WithClickable(isClickable),
		element.WithTitle(info.Title),
		element.WithDescription(info.RoleDescription),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create element: %w", err)
	}

	return elem, nil
}

// matchesFilter checks if an element matches the given filter criteria.
func (a *Adapter) matchesFilter(elem *element.Element, filter ports.ElementFilter) bool {
	// Check minimum size
	bounds := elem.Bounds()
	if bounds.Dx() < filter.MinSize.X || bounds.Dy() < filter.MinSize.Y {
		return false
	}

	// Check role inclusion
	if len(filter.Roles) > 0 {
		found := false
		for _, role := range filter.Roles {
			if elem.Role() == role {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check role exclusion
	for _, role := range filter.ExcludeRoles {
		if elem.Role() == role {
			return false
		}
	}

	return true
}

// Ensure Adapter implements ports.AccessibilityPort
var _ ports.AccessibilityPort = (*Adapter)(nil)
