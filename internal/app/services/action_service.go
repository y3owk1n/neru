package services

import (
	"context"
	"image"
	"strings"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/ports"
	"go.uber.org/zap"
)

// ActionService handles executing actions on UI elements.
type ActionService struct {
	accessibility ports.AccessibilityPort
	overlay       ports.OverlayPort
	config        config.ActionConfig
	keyBindings   config.ActionKeyBindingsCfg
	moveMouseStep int
	logger        *zap.Logger
}

// NewActionService creates a new action service.
func NewActionService(
	accessibility ports.AccessibilityPort,
	overlay ports.OverlayPort,
	actionConfig config.ActionConfig,
	keyBindings config.ActionKeyBindingsCfg,
	moveMouseStep int,
	logger *zap.Logger,
) *ActionService {
	return &ActionService{
		accessibility: accessibility,
		overlay:       overlay,
		config:        actionConfig,
		keyBindings:   keyBindings,
		moveMouseStep: moveMouseStep,
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

		return core.WrapActionFailed(performActionErr, actionType.String())
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
		return core.WrapConfigFailed(actionTypeErr, "validate action type")
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

		return core.WrapActionFailed(performActionErr, actionType.String()+" at point")
	}

	return nil
}

// IsFocusedAppExcluded checks if the currently focused application is in the exclusion list.
func (s *ActionService) IsFocusedAppExcluded(ctx context.Context) (bool, error) {
	bundleID, bundleIDErr := s.accessibility.FocusedAppBundleID(ctx)
	if bundleIDErr != nil {
		return false, core.WrapAccessibilityFailed(bundleIDErr, "get focused app bundle ID")
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

// MoveCursorToElement moves the cursor to the center of the specified element.
func (s *ActionService) MoveCursorToElement(
	ctx context.Context,
	element *element.Element,
) error {
	center := element.Center()

	return s.accessibility.MoveCursorToPoint(ctx, center, false)
}

// MoveCursorToPoint moves the cursor to the specified point.
func (s *ActionService) MoveCursorToPoint(ctx context.Context, point image.Point) error {
	return s.accessibility.MoveCursorToPoint(ctx, point, false)
}

// MoveMouseTo moves the mouse cursor to the specified coordinates (absolute).
// Coordinates are clamped to the screen bounds.
// If bypassSmooth is true, smooth cursor is bypassed for keyboard-driven movements.
func (s *ActionService) MoveMouseTo(
	ctx context.Context,
	targetX, targetY int,
	bypassSmooth ...bool,
) error {
	screenBounds, err := s.accessibility.ScreenBounds(ctx)
	if err != nil {
		return err
	}

	clampedX := min(max(targetX, screenBounds.Min.X), screenBounds.Max.X)
	clampedY := min(max(targetY, screenBounds.Min.Y), screenBounds.Max.Y)

	point := image.Point{X: clampedX, Y: clampedY}

	shouldBypass := len(bypassSmooth) > 0 && bypassSmooth[0]

	s.logger.Info("Moving mouse cursor",
		zap.Int("x", clampedX),
		zap.Int("y", clampedY),
		zap.Bool("clamped", clampedX != targetX || clampedY != targetY),
		zap.Bool("bypassSmooth", shouldBypass),
	)

	return s.accessibility.MoveCursorToPoint(ctx, point, shouldBypass)
}

// MoveMouseRelative moves the mouse cursor by the specified delta from the current position.
func (s *ActionService) MoveMouseRelative(ctx context.Context, deltaX, deltaY int) error {
	cursorPos, err := s.accessibility.CursorPosition(ctx)
	if err != nil {
		return err
	}

	return s.MoveMouseTo(ctx, cursorPos.X+deltaX, cursorPos.Y+deltaY, true)
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
		s.logger.Debug("Unknown action key",
			zap.String("mode", mode),
			zap.String("key", key))

		return false
	}

	s.logger.Info("Performing action",
		zap.String("mode", mode),
		zap.String("action", logMsg))

	// Perform action
	performActionErr := s.PerformAction(ctx, act, cursorPos)
	if performActionErr != nil {
		s.logger.Error("Failed to perform action", zap.Error(performActionErr))
	}

	return true
}

// Health checks the health of the service's dependencies.
func (s *ActionService) Health(ctx context.Context) map[string]error {
	return map[string]error{
		"accessibility": s.accessibility.Health(ctx),
		"overlay":       s.overlay.Health(ctx),
	}
}

// IsDirectActionKey checks if the given key is a direct action keybinding.
func (s *ActionService) IsDirectActionKey(key string) bool {
	_, _, ok := s.getActionMapping(key)

	return ok
}

// IsMoveMouseKey checks if the given key is a move mouse keybinding.
func (s *ActionService) IsMoveMouseKey(key string) bool {
	keyLower := strings.ToLower(key)

	return keyLower == strings.ToLower(s.keyBindings.MoveMouseUp) ||
		keyLower == strings.ToLower(s.keyBindings.MoveMouseDown) ||
		keyLower == strings.ToLower(s.keyBindings.MoveMouseLeft) ||
		keyLower == strings.ToLower(s.keyBindings.MoveMouseRight)
}

// HandleDirectActionKey processes a direct action key and performs the corresponding action.
// Returns true if the key was handled as a direct action, false otherwise.
func (s *ActionService) HandleDirectActionKey(ctx context.Context, key string) bool {
	actionString, logMsg, ok := s.getActionMapping(key)
	if !ok {
		return false
	}

	keyLower := strings.ToLower(key)

	if actionString == string(domain.ActionNameMoveMouseRelative) {
		var deltaX, deltaY int

		switch keyLower {
		case strings.ToLower(s.keyBindings.MoveMouseUp):
			deltaY = -s.moveMouseStep
		case strings.ToLower(s.keyBindings.MoveMouseDown):
			deltaY = s.moveMouseStep
		case strings.ToLower(s.keyBindings.MoveMouseLeft):
			deltaX = -s.moveMouseStep
		case strings.ToLower(s.keyBindings.MoveMouseRight):
			deltaX = s.moveMouseStep
		default:
			return false
		}

		s.logger.Info("Performing move mouse relative",
			zap.String("action", logMsg),
			zap.Int("dx", deltaX),
			zap.Int("dy", deltaY),
			zap.Int("step", s.moveMouseStep),
		)

		moveErr := s.MoveMouseRelative(ctx, deltaX, deltaY)
		if moveErr != nil {
			s.logger.Error("Failed to move mouse relative", zap.Error(moveErr))
		}

		return true
	}

	cursorPos, cursorPosErr := s.CursorPosition(ctx)
	if cursorPosErr != nil {
		s.logger.Error("Failed to get cursor position", zap.Error(cursorPosErr))

		return false
	}

	s.logger.Info("Performing direct action",
		zap.String("action", logMsg),
		zap.Int("x", cursorPos.X),
		zap.Int("y", cursorPos.Y))

	performActionErr := s.PerformAction(ctx, actionString, cursorPos)
	if performActionErr != nil {
		s.logger.Error("Failed to perform direct action", zap.Error(performActionErr))
	}

	return true
}

// getActionMapping returns the action string and log message for an action key.
func (s *ActionService) getActionMapping(key string) (string, string, bool) {
	if key == "" {
		return "", "", false
	}

	normalizedKey := key
	// Normalize carriage return to "Return"
	if key == "\r" {
		normalizedKey = "Return"
	}

	// Check direct match first
	if s.matchActionKey(normalizedKey) {
		return s.getActionForBinding(normalizedKey)
	}

	// If key is a single uppercase letter (A-Z), check if Shift+Key is configured
	// This handles the case where Shift+Letter is pressed but C code sends uppercase letter
	if len(normalizedKey) == 1 {
		r := rune(normalizedKey[0])
		if r >= 'A' && r <= 'Z' {
			shiftKey := "Shift+" + normalizedKey
			if s.matchActionKey(shiftKey) {
				return s.getActionForBinding(shiftKey)
			}
		}
	}

	return "", "", false
}

// matchActionKey checks if the key matches any configured binding.
func (s *ActionService) matchActionKey(key string) bool {
	keyLower := strings.ToLower(key)

	return keyLower == strings.ToLower(s.keyBindings.LeftClick) ||
		keyLower == strings.ToLower(s.keyBindings.RightClick) ||
		keyLower == strings.ToLower(s.keyBindings.MiddleClick) ||
		keyLower == strings.ToLower(s.keyBindings.MouseDown) ||
		keyLower == strings.ToLower(s.keyBindings.MouseUp) ||
		keyLower == strings.ToLower(s.keyBindings.MoveMouseUp) ||
		keyLower == strings.ToLower(s.keyBindings.MoveMouseDown) ||
		keyLower == strings.ToLower(s.keyBindings.MoveMouseLeft) ||
		keyLower == strings.ToLower(s.keyBindings.MoveMouseRight)
}

// getActionForBinding returns the action for a matching binding.
func (s *ActionService) getActionForBinding(binding string) (string, string, bool) {
	bindingLower := strings.ToLower(binding)

	bindings := []struct {
		config string
		action domain.ActionName
		logMsg string
	}{
		{s.keyBindings.LeftClick, domain.ActionNameLeftClick, "Left click"},
		{s.keyBindings.RightClick, domain.ActionNameRightClick, "Right click"},
		{s.keyBindings.MiddleClick, domain.ActionNameMiddleClick, "Middle click"},
		{s.keyBindings.MouseDown, domain.ActionNameMouseDown, "Mouse down"},
		{s.keyBindings.MouseUp, domain.ActionNameMouseUp, "Mouse up"},
		{s.keyBindings.MoveMouseUp, domain.ActionNameMoveMouseRelative, "Move mouse up"},
		{s.keyBindings.MoveMouseDown, domain.ActionNameMoveMouseRelative, "Move mouse down"},
		{s.keyBindings.MoveMouseLeft, domain.ActionNameMoveMouseRelative, "Move mouse left"},
		{s.keyBindings.MoveMouseRight, domain.ActionNameMoveMouseRelative, "Move mouse right"},
	}

	for _, b := range bindings {
		if bindingLower == strings.ToLower(b.config) {
			return string(b.action), b.logMsg, true
		}
	}

	return "", "", false
}
