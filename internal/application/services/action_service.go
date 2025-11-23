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
	cfg config.ActionConfig,
	logger *zap.Logger,
) *ActionService {
	return &ActionService{
		accessibility: accessibility,
		overlay:       overlay,
		config:        cfg,
		logger:        logger,
	}
}

// ExecuteAction performs the specified action on the given element.
func (s *ActionService) ExecuteAction(
	ctx context.Context,
	elem *element.Element,
	actionType action.Type,
) error {
	s.logger.Info("Executing action",
		zap.String("action", actionType.String()),
		zap.String("element_id", string(elem.ID())),
		zap.String("element_role", string(elem.Role())))

	err := s.accessibility.PerformAction(ctx, elem, actionType)
	if err != nil {
		s.logger.Error("Failed to perform action",
			zap.Error(err),
			zap.String("action", actionType.String()))
		return fmt.Errorf("failed to perform %s action: %w", actionType, err)
	}

	s.logger.Info("Action executed successfully",
		zap.String("action", actionType.String()))
	return nil
}

// PerformAction executes an action at the specified point.
// This parses the action string to a domain type and delegates to the accessibility port.
func (s *ActionService) PerformAction(
	ctx context.Context,
	actionStr string,
	point image.Point,
) error {
	// Parse action string to domain type
	actionType, err := action.ParseType(actionStr)
	if err != nil {
		return fmt.Errorf("invalid action type: %w", err)
	}

	s.logger.Info("Performing action at point",
		zap.String("action", actionType.String()),
		zap.Int("x", point.X),
		zap.Int("y", point.Y))

	err = s.accessibility.PerformActionAtPoint(ctx, actionType, point)
	if err != nil {
		s.logger.Error("Failed to perform action at point",
			zap.Error(err),
			zap.String("action", actionType.String()))
		return fmt.Errorf("failed to perform %s action at point: %w", actionType, err)
	}

	return nil
}

// IsFocusedAppExcluded checks if the currently focused application is in the exclusion list.
func (s *ActionService) IsFocusedAppExcluded(ctx context.Context) (bool, error) {
	bundleID, err := s.accessibility.GetFocusedAppBundleID(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get focused app bundle ID: %w", err)
	}

	isExcluded := s.accessibility.IsAppExcluded(ctx, bundleID)
	if isExcluded {
		s.logger.Info("Focused app is excluded", zap.String("bundle_id", bundleID))
	}

	return isExcluded, nil
}

// GetFocusedAppBundleID returns the bundle ID of the currently focused application.
func (s *ActionService) GetFocusedAppBundleID(ctx context.Context) (string, error) {
	return s.accessibility.GetFocusedAppBundleID(ctx)
}

// ShowActionHighlight displays the action mode highlight around the active screen.
func (s *ActionService) ShowActionHighlight(ctx context.Context) error {
	// Get active screen bounds
	bounds, err := s.accessibility.GetScreenBounds(ctx)
	if err != nil {
		return fmt.Errorf("failed to get screen bounds: %w", err)
	}

	// Draw highlight using overlay
	err = s.overlay.DrawActionHighlight(
		ctx,
		bounds,
		s.config.HighlightColor,
		s.config.HighlightWidth,
	)
	if err != nil {
		return fmt.Errorf("failed to draw action highlight: %w", err)
	}

	s.logger.Debug("Action highlight displayed")
	return nil
}

// MoveCursorToElement moves the cursor to the center of the specified element.
func (s *ActionService) MoveCursorToElement(ctx context.Context, elem *element.Element) error {
	center := elem.Center()
	return s.accessibility.MoveCursorToPoint(ctx, center)
}

// MoveCursorToPoint moves the cursor to the specified point.
func (s *ActionService) MoveCursorToPoint(ctx context.Context, point image.Point) error {
	return s.accessibility.MoveCursorToPoint(ctx, point)
}

// GetCursorPosition returns the current cursor position.
func (s *ActionService) GetCursorPosition(ctx context.Context) (image.Point, error) {
	return s.accessibility.GetCursorPosition(ctx)
}
