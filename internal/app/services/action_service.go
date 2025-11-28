package services

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/ports"
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
	ctx context.Context,
	element *element.Element,
	actionType action.Type,
) error {
	s.logger.Info("Executing action",
		zap.String("action", actionType.String()),
		zap.String("element_id", string(element.ID())),
		zap.String("element_role", string(element.Role())))

	performActionErr := s.accessibility.PerformAction(ctx, element, actionType)
	if performActionErr != nil {
		s.logger.Error("Failed to perform action",
			zap.Error(performActionErr),
			zap.String("action", actionType.String()))

		return derrors.Wrap(performActionErr, derrors.CodeActionFailed, "failed to perform action")
	}

	s.logger.Info("Action executed successfully",
		zap.String("action", actionType.String()))

	return nil
}

// PerformAction executes an action at the specified point.
// This parses the action string to a domain type and delegates to the accessibility port.
func (s *ActionService) PerformAction(
	ctx context.Context,
	actionString string,
	point image.Point,
) error {
	// Parse action string to domain type
	actionType, actionTypeErr := action.ParseType(actionString)
	if actionTypeErr != nil {
		return derrors.Wrap(actionTypeErr, derrors.CodeInvalidInput, "invalid action type")
	}

	s.logger.Info("Performing action at point",
		zap.String("action", actionType.String()),
		zap.Int("x", point.X),
		zap.Int("y", point.Y))

	performActionErr := s.accessibility.PerformActionAtPoint(ctx, actionType, point)
	if performActionErr != nil {
		s.logger.Error("Failed to perform action at point",
			zap.Error(performActionErr),
			zap.String("action", actionType.String()))

		return derrors.Wrap(
			performActionErr,
			derrors.CodeActionFailed,
			"failed to perform action at point",
		)
	}

	return nil
}

// IsFocusedAppExcluded checks if the currently focused application is in the exclusion list.
func (s *ActionService) IsFocusedAppExcluded(ctx context.Context) (bool, error) {
	bundleID, bundleIDErr := s.accessibility.FocusedAppBundleID(ctx)
	if bundleIDErr != nil {
		return false, derrors.Wrap(
			bundleIDErr,
			derrors.CodeAccessibilityFailed,
			"failed to get focused app bundle ID",
		)
	}

	isExcluded := s.accessibility.IsAppExcluded(ctx, bundleID)
	if isExcluded {
		s.logger.Info("Focused app is excluded", zap.String("bundle_id", bundleID))
	}

	return isExcluded, nil
}

// FocusedAppBundleID returns the bundle ID of the currently focused application.
func (s *ActionService) FocusedAppBundleID(ctx context.Context) (string, error) {
	return s.accessibility.FocusedAppBundleID(ctx)
}

// ShowActionHighlight displays the action mode highlight around the active screen.
func (s *ActionService) ShowActionHighlight(ctx context.Context) error {
	// Get active screen screenBounds
	screenBounds, screenBoundsErr := s.accessibility.ScreenBounds(ctx)
	if screenBoundsErr != nil {
		return derrors.Wrap(
			screenBoundsErr,
			derrors.CodeAccessibilityFailed,
			"failed to get screen bounds",
		)
	}

	// Draw highlight using overlay
	DrawActionHighlightErr := s.overlay.DrawActionHighlight(
		ctx,
		screenBounds,
		s.config.HighlightColor,
		s.config.HighlightWidth,
	)
	if DrawActionHighlightErr != nil {
		return derrors.Wrap(
			DrawActionHighlightErr,
			derrors.CodeOverlayFailed,
			"failed to draw action highlight",
		)
	}

	s.logger.Debug("Action highlight displayed")

	return nil
}

// MoveCursorToElement moves the cursor to the center of the specified element.
func (s *ActionService) MoveCursorToElement(
	ctx context.Context,
	element *element.Element,
) error {
	center := element.Center()

	return s.accessibility.MoveCursorToPoint(ctx, center)
}

// MoveCursorToPoint moves the cursor to the specified point.
func (s *ActionService) MoveCursorToPoint(ctx context.Context, point image.Point) error {
	return s.accessibility.MoveCursorToPoint(ctx, point)
}

// CursorPosition returns the current cursor position.
func (s *ActionService) CursorPosition(ctx context.Context) (image.Point, error) {
	return s.accessibility.CursorPosition(ctx)
}

// HandleActionKey processes an action key and performs the corresponding action at the current cursor position.
// Returns true if the key was handled as an action key, false otherwise.
func (s *ActionService) HandleActionKey(ctx context.Context, key string, mode string) bool {
	cursorPos, cursorPosErr := s.CursorPosition(ctx)
	if cursorPosErr != nil {
		s.logger.Error("Failed to get cursor position", zap.Error(cursorPosErr))

		return false
	}

	act, logMsg, ok := s.getActionMapping(key)
	if !ok {
		s.logger.Debug("Unknown "+mode+" action key", zap.String("key", key))

		return false
	}

	s.logger.Info(mode + " action: " + logMsg)

	// Perform action
	performActionErr := s.PerformAction(ctx, act, cursorPos)
	if performActionErr != nil {
		s.logger.Error("Failed to perform action", zap.Error(performActionErr))
	}

	return true
}

// getActionMapping returns the action name and log message for a given key.
func (s *ActionService) getActionMapping(key string) (string, string, bool) {
	switch key {
	case s.config.LeftClickKey:
		return string(domain.ActionNameLeftClick), "Left click", true
	case s.config.RightClickKey:
		return string(domain.ActionNameRightClick), "Right click", true
	case s.config.MiddleClickKey:
		return string(domain.ActionNameMiddleClick), "Middle click", true
	case s.config.MouseDownKey:
		return string(domain.ActionNameMouseDown), "Mouse down", true
	case s.config.MouseUpKey:
		return string(domain.ActionNameMouseUp), "Mouse up", true
	default:
		return "", "", false
	}
}
