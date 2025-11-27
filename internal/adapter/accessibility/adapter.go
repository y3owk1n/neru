package accessibility

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
	derrors "github.com/y3owk1n/neru/internal/errors"
	"go.uber.org/zap"
)

// Adapter implements ports.AccessibilityPort by wrapping the AXClient.
// It converts between domain models and infrastructure types.
type Adapter struct {
	// logger for adapter.
	logger          *zap.Logger
	client          AXClient
	excludedBundles map[string]bool
	// clickableRoles is the list of clickable roles.
	clickableRoles []string
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

// Logger returns the logger for the adapter.
// It is used for testing mainly.
func (a *Adapter) Logger() *zap.Logger {
	return a.logger
}

// ClickableRoles returns the list of clickable roles.
// It is used for testing mainly.
func (a *Adapter) ClickableRoles() []string {
	return a.clickableRoles
}

// ClickableElements retrieves all clickable UI elements matching the filter.
func (a *Adapter) ClickableElements(
	ctx context.Context,
	filter ports.ElementFilter,
) ([]*element.Element, error) {
	// Check context
	err := a.checkContext(ctx)
	if err != nil {
		return nil, err
	}

	a.logger.Debug("Getting clickable elements", zap.Any("filter", filter))

	frontmostWindow, frontmostWindowErr := a.client.FrontmostWindow()
	if frontmostWindowErr != nil {
		return nil, derrors.New(derrors.CodeAccessibilityFailed, "failed to get frontmost window")
	}
	defer frontmostWindow.Release()

	clickableNodes, clickableNodesErr := a.client.ClickableNodes(
		frontmostWindow,
		filter.IncludeOffscreen,
	)
	if clickableNodesErr != nil {
		return nil, derrors.Wrap(
			clickableNodesErr,
			derrors.CodeAccessibilityFailed,
			"failed to get clickable nodes",
		)
	}

	a.logger.Debug("Found clickable nodes", zap.Int("count", len(clickableNodes)))

	// Convert to domain elements
	elements, processErr := a.processClickableNodes(ctx, clickableNodes, filter)
	if processErr != nil {
		return nil, processErr
	}

	a.logger.Info("Converted frontmost window elements", zap.Int("count", len(elements)))

	// Add supplementary elements based on filter
	elements = a.addSupplementaryElements(ctx, elements, filter)

	a.logger.Info("Total elements after supplementary collection", zap.Int("count", len(elements)))

	return elements, nil
}

// PerformAction executes an action on the specified element.
func (a *Adapter) PerformAction(
	ctx context.Context,
	element *element.Element,
	actionType action.Type,
) error {
	// Check context
	select {
	case <-ctx.Done():
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
	}

	a.logger.Info("Performing action",
		zap.String("action", actionType.String()),
		zap.String("element_id", string(element.ID())))

	center := element.Center()

	restoreCursor := a.getRestoreCursor()

	// Perform the action via client
	performActionErr := a.client.PerformAction(actionType, center, restoreCursor)
	if performActionErr != nil {
		return derrors.Wrap(performActionErr, derrors.CodeActionFailed, "failed to perform action")
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
	err := a.checkContext(ctx)
	if err != nil {
		return err
	}

	a.logger.Info("Performing action at point",
		zap.String("action", actionType.String()),
		zap.Int("x", point.X),
		zap.Int("y", point.Y))

	restoreCursor := a.getRestoreCursor()

	// Perform the action via client
	performActionErr := a.client.PerformAction(actionType, point, restoreCursor)
	if performActionErr != nil {
		return derrors.Wrap(
			performActionErr,
			derrors.CodeActionFailed,
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
		return derrors.Wrap(scrollErr, derrors.CodeActionFailed, "failed to scroll")
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

// CursorPosition returns the current cursor position.
func (a *Adapter) CursorPosition(_ context.Context) (image.Point, error) {
	pos := a.client.CursorPosition()
	a.logger.Debug("Got cursor position",
		zap.Int("x", pos.X),
		zap.Int("y", pos.Y))

	return pos, nil
}

// FocusedAppBundleID returns the bundle ID of the currently focused application.
func (a *Adapter) FocusedAppBundleID(ctx context.Context) (string, error) {
	// Check context
	err := a.checkContext(ctx)
	if err != nil {
		return "", err
	}

	focusedApp, focusedAppErr := a.client.FocusedApplication()
	if focusedAppErr != nil {
		return "", derrors.New(derrors.CodeAccessibilityFailed, "failed to get focused application")
	}
	defer focusedApp.Release()

	bundleID := focusedApp.BundleIdentifier()
	if bundleID == "" {
		return "", derrors.New(derrors.CodeAccessibilityFailed, "failed to get bundle ID")
	}

	return bundleID, nil
}

// IsAppExcluded checks if the given bundle ID is in the exclusion list.
func (a *Adapter) IsAppExcluded(_ context.Context, bundleID string) bool {
	return a.excludedBundles[bundleID]
}

// ScreenBounds returns the bounds of the active screen.
func (a *Adapter) ScreenBounds(ctx context.Context) (image.Rectangle, error) {
	// Check context
	err := a.checkContext(ctx)
	if err != nil {
		return image.Rectangle{}, err
	}

	return a.client.ActiveScreenBounds(), nil
}

// CheckPermissions verifies that accessibility permissions are granted.
func (a *Adapter) CheckPermissions(ctx context.Context) error {
	// Check context
	err := a.checkContext(ctx)
	if err != nil {
		return err
	}

	if !a.client.CheckPermissions() {
		return derrors.New(derrors.CodeAccessibilityDenied,
			"accessibility permissions not granted - please enable in System Preferences")
	}

	return nil
}

// Health checks if the accessibility permissions are granted.
func (a *Adapter) Health(ctx context.Context) error {
	return a.CheckPermissions(ctx)
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

// checkContext checks if the context is canceled and returns an error if so.
func (a *Adapter) checkContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
		return nil
	}
}

// getRestoreCursor retrieves the restore cursor setting from global config.
func (a *Adapter) getRestoreCursor() bool {
	cfg := config.Global()

	return cfg != nil && cfg.General.RestoreCursorPosition
}

// processClickableNodes converts and filters clickable nodes to domain elements.
func (a *Adapter) processClickableNodes(
	ctx context.Context,
	clickableNodes []AXNode,
	filter ports.ElementFilter,
) ([]*element.Element, error) {
	elements := make([]*element.Element, 0, len(clickableNodes))
	for index, node := range clickableNodes {
		// Check context periodically
		if index%100 == 0 {
			err := a.checkContext(ctx)
			if err != nil {
				return nil, err
			}
		}

		elem, err := a.convertToDomainElement(node)
		if err != nil {
			a.logger.Warn("Failed to convert element", zap.Error(err))

			continue
		}

		// Apply filter
		if a.MatchesFilter(elem, filter) {
			elements = append(elements, elem)
		}
	}

	return elements, nil
}

// Ensure Adapter implements ports.AccessibilityPort.
var _ ports.AccessibilityPort = (*Adapter)(nil)
