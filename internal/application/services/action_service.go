package services

import (
	"context"
	"fmt"
	"image"

	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
	"go.uber.org/zap"
)

// ActionService handles executing actions on UI elements.
type ActionService struct {
	accessibility ports.AccessibilityPort
	overlay       ports.OverlayPort
	config        config.ActionConfig
	logger        *zap.Logger
}

// NewActionService creates a new action service.
func NewActionService(
	accessibility ports.AccessibilityPort,
	overlay ports.OverlayPort,
	config config.ActionConfig,
	logger *zap.Logger,
) *ActionService {
	return &ActionService{
		accessibility: accessibility,
		overlay:       overlay,
		config:        config,
		logger:        logger,
	}
}

// ExecuteAction performs the specified action on the given element.
func (s *ActionService) ExecuteAction(
	context context.Context,
	element *element.Element,
	actionType action.Type,
) error {
	s.logger.Info("Executing action",
		zap.String("action", actionType.String()),
		zap.String("element_id", string(element.ID())),
		zap.String("element_role", string(element.Role())))

	performActionErr := s.accessibility.PerformAction(context, element, actionType)
	if performActionErr != nil {
		s.logger.Error("Failed to perform action",
			zap.Error(performActionErr),
			zap.String("action", actionType.String()))

		return fmt.Errorf("failed to perform %s action: %w", actionType, performActionErr)
	}

	s.logger.Info("Action executed successfully",
		zap.String("action", actionType.String()))

	return nil
}

// PerformAction executes an action at the specified point.
// This parses the action string to a domain type and delegates to the accessibility port.
func (s *ActionService) PerformAction(
	context context.Context,
	actionString string,
	point image.Point,
) error {
	// Parse action string to domain type
	actionType, actionTypeErr := action.ParseType(actionString)
	if actionTypeErr != nil {
		return fmt.Errorf("invalid action type: %w", actionTypeErr)
	}

	s.logger.Info("Performing action at point",
		zap.String("action", actionType.String()),
		zap.Int("x", point.X),
		zap.Int("y", point.Y))

	performActionErr := s.accessibility.PerformActionAtPoint(context, actionType, point)
	if performActionErr != nil {
		s.logger.Error("Failed to perform action at point",
			zap.Error(performActionErr),
			zap.String("action", actionType.String()))

		return fmt.Errorf("failed to perform %s action at point: %w", actionType, performActionErr)
	}

	return nil
}

// IsFocusedAppExcluded checks if the currently focused application is in the exclusion list.
func (s *ActionService) IsFocusedAppExcluded(context context.Context) (bool, error) {
	bundleID, bundleIDErr := s.accessibility.GetFocusedAppBundleID(context)
	if bundleIDErr != nil {
		return false, fmt.Errorf("failed to get focused app bundle ID: %w", bundleIDErr)
	}

	isExcluded := s.accessibility.IsAppExcluded(context, bundleID)
	if isExcluded {
		s.logger.Info("Focused app is excluded", zap.String("bundle_id", bundleID))
	}

	return isExcluded, nil
}

// GetFocusedAppBundleID returns the bundle ID of the currently focused application.
func (s *ActionService) GetFocusedAppBundleID(context context.Context) (string, error) {
	return s.accessibility.GetFocusedAppBundleID(context)
}

// ShowActionHighlight displays the action mode highlight around the active screen.
func (s *ActionService) ShowActionHighlight(context context.Context) error {
	// Get active screen screenBounds
	screenBounds, screenBoundsErr := s.accessibility.GetScreenBounds(context)
	if screenBoundsErr != nil {
		return fmt.Errorf("failed to get screen bounds: %w", screenBoundsErr)
	}

	// Draw highlight using overlay
	DrawActionHighlightErr := s.overlay.DrawActionHighlight(
		context,
		screenBounds,
		s.config.HighlightColor,
		s.config.HighlightWidth,
	)
	if DrawActionHighlightErr != nil {
		return fmt.Errorf("failed to draw action highlight: %w", DrawActionHighlightErr)
	}

	s.logger.Debug("Action highlight displayed")

	return nil
}

// MoveCursorToElement moves the cursor to the center of the specified element.
func (s *ActionService) MoveCursorToElement(
	context context.Context,
	element *element.Element,
) error {
	center := element.Center()

	return s.accessibility.MoveCursorToPoint(context, center)
}

// MoveCursorToPoint moves the cursor to the specified point.
func (s *ActionService) MoveCursorToPoint(context context.Context, point image.Point) error {
	return s.accessibility.MoveCursorToPoint(context, point)
}

// GetCursorPosition returns the current cursor position.
func (s *ActionService) GetCursorPosition(context context.Context) (image.Point, error) {
	return s.accessibility.GetCursorPosition(context)
}
