package accessibility

import (
	"context"
	"fmt"
	"image"
	"slices"

	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/errors"
	"go.uber.org/zap"
)

// Adapter implements ports.AccessibilityPort by wrapping the AXClient.
// It converts between domain models and infrastructure types.
type Adapter struct {
	logger          *zap.Logger
	client          AXClient
	excludedBundles map[string]bool
	clickableRoles  []string
}

// NewAdapter creates a new accessibility adapter.
func NewAdapter(
	logger *zap.Logger,
	excludedBundles []string,
	clickableRoles []string,
	client AXClient,
) *Adapter {
	excludedMap := make(map[string]bool, len(excludedBundles))
	for _, bundle := range excludedBundles {
		excludedMap[bundle] = true
	}

	return &Adapter{
		logger:          logger,
		client:          client,
		excludedBundles: excludedMap,
		clickableRoles:  clickableRoles,
	}
}

// GetClickableElements retrieves all clickable UI elements matching the filter.
func (a *Adapter) GetClickableElements(
	context context.Context,
	filter ports.ElementFilter,
) ([]*element.Element, error) {
	// Check context
	select {
	case <-context.Done():
		return nil, context.Err()
	default:
	}

	a.logger.Debug("Getting clickable elements", zap.Any("filter", filter))

	// Get frontmost frontmostWindow
	frontmostWindow, frontmostWindowErr := a.client.GetFrontmostWindow()
	if frontmostWindowErr != nil {
		return nil, errors.New(errors.CodeAccessibilityFailed, "failed to get frontmost window")
	}
	defer frontmostWindow.Release()

	// Get clickable nodes via client
	clickableNodes, clickableNodesErr := a.client.GetClickableNodes(
		frontmostWindow,
		filter.IncludeOffscreen,
	)
	if clickableNodesErr != nil {
		return nil, errors.Wrap(
			clickableNodesErr,
			errors.CodeAccessibilityFailed,
			"failed to get clickable nodes",
		)
	}

	a.logger.Debug("Found clickable nodes", zap.Int("count", len(clickableNodes)))

	// Convert to domain elements
	elements := make([]*element.Element, 0, len(clickableNodes))
	for index, node := range clickableNodes {
		// Check context periodically
		if index%100 == 0 {
			select {
			case <-context.Done():
				return nil, context.Err()
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

	a.logger.Info("Converted frontmost window elements", zap.Int("count", len(elements)))

	// Add supplementary elements based on filter
	elements = a.addSupplementaryElements(context, elements, filter)

	a.logger.Info("Total elements after supplementary collection", zap.Int("count", len(elements)))

	return elements, nil
}

// PerformAction executes an action on the specified element.
func (a *Adapter) PerformAction(
	context context.Context,
	element *element.Element,
	actionType action.Type,
) error {
	// Check context
	select {
	case <-context.Done():
		return context.Err()
	default:
	}

	a.logger.Info("Performing action",
		zap.String("action", actionType.String()),
		zap.String("element_id", string(element.ID())))

	// Get the center point of the element
	center := element.Center()

	// Get restore cursor setting from config
	config := config.Global()
	restoreCursor := config != nil && config.General.RestoreCursorPosition

	// Perform the action via client
	performActionErr := a.client.PerformAction(actionType, center, restoreCursor)
	if performActionErr != nil {
		return errors.Wrap(performActionErr, errors.CodeActionFailed, "failed to perform action")
	}

	return nil
}

// PerformActionAtPoint executes an action at the specified point.
func (a *Adapter) PerformActionAtPoint(
	context context.Context,
	actionType action.Type,
	point image.Point,
) error {
	// Check context
	select {
	case <-context.Done():
		return context.Err()
	default:
	}

	a.logger.Info("Performing action at point",
		zap.String("action", actionType.String()),
		zap.Int("x", point.X),
		zap.Int("y", point.Y))

	// Get restore cursor setting from config
	config := config.Global()
	restoreCursor := config != nil && config.General.RestoreCursorPosition

	// Perform the action via client
	performActionErr := a.client.PerformAction(actionType, point, restoreCursor)
	if performActionErr != nil {
		return errors.Wrap(
			performActionErr,
			errors.CodeActionFailed,
			"failed to perform action at point",
		)
	}

	return nil
}

// Scroll performs a scroll action at the current cursor position.
func (a *Adapter) Scroll(_ context.Context, deltaX, deltaY int) error {
	a.logger.Debug("Performing scroll",
		zap.Int("deltaX", deltaX),
		zap.Int("deltaY", deltaY))

	scrollErr := a.client.Scroll(deltaX, deltaY)
	if scrollErr != nil {
		return errors.Wrap(scrollErr, errors.CodeActionFailed, "failed to scroll")
	}

	a.logger.Debug("Scroll completed")

	return nil
}

// MoveCursorToPoint moves the mouse cursor to the specified point.
func (a *Adapter) MoveCursorToPoint(_ context.Context, point image.Point) error {
	a.logger.Debug("Moving cursor to point",
		zap.Int("x", point.X),
		zap.Int("y", point.Y))

	a.client.MoveMouse(point)

	return nil
}

// GetCursorPosition returns the current cursor position.
func (a *Adapter) GetCursorPosition(_ context.Context) (image.Point, error) {
	pos := a.client.GetCursorPosition()
	a.logger.Debug("Got cursor position",
		zap.Int("x", pos.X),
		zap.Int("y", pos.Y))

	return pos, nil
}

// GetFocusedAppBundleID returns the bundle ID of the currently focused application.
func (a *Adapter) GetFocusedAppBundleID(context context.Context) (string, error) {
	// Check context
	select {
	case <-context.Done():
		return "", context.Err()
	default:
	}

	focusedApp, focusedAppErr := a.client.GetFocusedApplication()
	if focusedAppErr != nil {
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
func (a *Adapter) IsAppExcluded(_ context.Context, bundleID string) bool {
	return a.excludedBundles[bundleID]
}

// GetScreenBounds returns the bounds of the active screen.
func (a *Adapter) GetScreenBounds(context context.Context) (image.Rectangle, error) {
	// Check context
	select {
	case <-context.Done():
		return image.Rectangle{}, context.Err()
	default:
	}

	return a.client.GetActiveScreenBounds(), nil
}

// CheckPermissions verifies that accessibility permissions are granted.
func (a *Adapter) CheckPermissions(context context.Context) error {
	// Check context
	select {
	case <-context.Done():
		return context.Err()
	default:
	}

	if !a.client.CheckPermissions() {
		return errors.New(errors.CodeAccessibilityDenied,
			"accessibility permissions not granted - please enable in System Preferences")
	}

	return nil
}

// Health checks if the accessibility permissions are granted.
func (a *Adapter) Health(context context.Context) error {
	return a.CheckPermissions(context)
}

// UpdateClickableRoles updates the list of clickable roles.
func (a *Adapter) UpdateClickableRoles(roles []string) {
	a.logger.Info("Updating clickable roles", zap.Int("count", len(roles)))
	a.clickableRoles = roles
	a.client.SetClickableRoles(roles)
}

// UpdateExcludedBundles updates the list of excluded bundle IDs.
func (a *Adapter) UpdateExcludedBundles(bundles []string) {
	a.logger.Info("Updating excluded bundles", zap.Int("count", len(bundles)))

	a.excludedBundles = make(map[string]bool, len(bundles))
	for _, bundle := range bundles {
		a.excludedBundles[bundle] = true
	}
}

// convertToDomainElement converts an AXNode to a domain Element.
func (a *Adapter) convertToDomainElement(node AXNode) (*element.Element, error) {
	if node == nil {
		return nil, errors.New(errors.CodeInvalidInput, "node is nil")
	}

	// Create element ID from unique identifier
	elementID := element.ID(node.GetID())

	// Get bounds
	bounds := node.GetBounds()

	// Convert role
	role := element.Role(node.GetRole())

	// Determine if clickable
	isClickable := node.IsClickable()

	// Create element with options
	element, elementErr := element.NewElement(
		elementID,
		bounds,
		role,
		element.WithClickable(isClickable),
		element.WithTitle(node.GetTitle()),
		element.WithDescription(node.GetDescription()),
	)
	if elementErr != nil {
		return nil, fmt.Errorf("failed to create element: %w", elementErr)
	}

	return element, nil
}

// matchesFilter checks if an element matches the given filter criteria.
func (a *Adapter) matchesFilter(element *element.Element, filter ports.ElementFilter) bool {
	// Check minimum size
	bounds := element.Bounds()
	if bounds.Dx() < filter.MinSize.X || bounds.Dy() < filter.MinSize.Y {
		return false
	}

	// Check role inclusion
	if len(filter.Roles) > 0 {
		found := slices.Contains(filter.Roles, element.Role())
		if !found {
			return false
		}
	}

	// Check role exclusion
	return !slices.Contains(filter.ExcludeRoles, element.Role())
}

// Ensure Adapter implements ports.AccessibilityPort.
var _ ports.AccessibilityPort = (*Adapter)(nil)
