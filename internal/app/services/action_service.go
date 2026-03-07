package services

import (
	"context"
	"image"
	"strings"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/ports"
	"go.uber.org/zap"
)

// ActionService handles executing actions on UI elements.
type ActionService struct {
	BaseService

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
		BaseService:   NewBaseService(accessibility, overlay),
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

// PerformActionAtPoint executes an action at the specified point.
// This parses the action string to a domain type and delegates to the accessibility port.
func (s *ActionService) PerformActionAtPoint(
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

// clampToScreenBounds clamps the given point so it stays within the screen bounds.
func clampToScreenBounds(p image.Point, bounds image.Rectangle) image.Point {
	return image.Point{
		X: min(max(p.X, bounds.Min.X), bounds.Max.X-1),
		Y: min(max(p.Y, bounds.Min.Y), bounds.Max.Y-1),
	}
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
		s.logger.Error("Failed to get screen bounds", zap.Error(err))

		return core.WrapAccessibilityFailed(err, "get screen bounds")
	}

	target := image.Point{X: targetX, Y: targetY}
	clamped := clampToScreenBounds(target, screenBounds)

	shouldBypass := len(bypassSmooth) > 0 && bypassSmooth[0]

	s.logger.Info("Moving mouse cursor",
		zap.Int("x", clamped.X),
		zap.Int("y", clamped.Y),
		zap.Bool("clamped", clamped != target),
		zap.Bool("bypassSmooth", shouldBypass),
	)

	return s.accessibility.MoveCursorToPoint(ctx, clamped, shouldBypass)
}

// MoveMouseRelative moves the mouse cursor by the specified delta from the current position.
func (s *ActionService) MoveMouseRelative(ctx context.Context, deltaX, deltaY int) error {
	cursorPos, err := s.accessibility.CursorPosition(ctx)
	if err != nil {
		s.logger.Error("Failed to get cursor position", zap.Error(err))

		return core.WrapAccessibilityFailed(err, "get cursor position")
	}

	return s.MoveMouseTo(ctx, cursorPos.X+deltaX, cursorPos.Y+deltaY, true)
}

// CursorPosition returns the current cursor position.
func (s *ActionService) CursorPosition(ctx context.Context) (image.Point, error) {
	return s.accessibility.CursorPosition(ctx)
}

// ScreenBounds returns the bounds of the active screen.
func (s *ActionService) ScreenBounds(ctx context.Context) (image.Rectangle, error) {
	return s.accessibility.ScreenBounds(ctx)
}

// MoveMouseToCenter moves the mouse cursor to the center of the active screen,
// optionally offset by the given delta values. It fetches screen bounds once
// for both center computation and clamping.
func (s *ActionService) MoveMouseToCenter(ctx context.Context, offsetX, offsetY int) error {
	screenBounds, err := s.accessibility.ScreenBounds(ctx)
	if err != nil {
		s.logger.Error("Failed to get screen bounds", zap.Error(err))

		return core.WrapAccessibilityFailed(err, "get screen bounds")
	}

	centerX := screenBounds.Min.X + screenBounds.Dx()/2 //nolint:mnd
	centerY := screenBounds.Min.Y + screenBounds.Dy()/2 //nolint:mnd
	target := image.Point{X: centerX + offsetX, Y: centerY + offsetY}
	clamped := clampToScreenBounds(target, screenBounds)

	s.logger.Info("Moving mouse cursor to center",
		zap.Int("x", clamped.X),
		zap.Int("y", clamped.Y),
		zap.Int("offsetX", offsetX),
		zap.Int("offsetY", offsetY),
		zap.Bool("clamped", clamped != target),
	)

	return s.accessibility.MoveCursorToPoint(ctx, clamped, false)
}

// HandleActionKey processes an action key and performs the corresponding action at the current cursor position.
// Returns true if the key was handled as an action key, false otherwise.
// Returns an error if the action failed to execute.
func (s *ActionService) HandleActionKey(
	ctx context.Context,
	key string,
	mode string,
) (bool, error) {
	act, logMsg, _, ok := s.getActionMapping(key)

	if !ok {
		s.logger.Debug("Unknown action key",
			zap.String("mode", mode),
			zap.String("key", key))

		return false, nil
	}

	cursorPos, cursorPosErr := s.CursorPosition(ctx)
	if cursorPosErr != nil {
		s.logger.Error("Failed to get cursor position", zap.Error(cursorPosErr))

		return true, core.WrapAccessibilityFailed(cursorPosErr, "get cursor position")
	}

	s.logger.Info("Performing action",
		zap.String("mode", mode),
		zap.String("action", logMsg))

	// Perform action
	performActionErr := s.PerformActionAtPoint(ctx, act, cursorPos)
	if performActionErr != nil {
		s.logger.Error("Failed to perform action", zap.Error(performActionErr))

		return true, performActionErr
	}

	return true, nil
}

// IsMoveMouseKey checks if the given key is a move mouse keybinding.
// It handles Shift+Letter normalization (e.g. "K" matching "Shift+K").
func (s *ActionService) IsMoveMouseKey(key string) bool {
	if key == "" {
		return false
	}

	bindings := []string{
		s.keyBindings.MoveMouseUp,
		s.keyBindings.MoveMouseDown,
		s.keyBindings.MoveMouseLeft,
		s.keyBindings.MoveMouseRight,
	}

	keysToCheck := []string{key}
	// If key is a single uppercase letter, also check against Shift+Key
	if len(key) == 1 {
		r := rune(key[0])
		if r >= 'A' && r <= 'Z' {
			keysToCheck = append(keysToCheck, "Shift+"+key)
		}
	}

	for _, binding := range bindings {
		if binding == "" {
			continue
		}

		for _, k := range keysToCheck {
			if strings.EqualFold(k, binding) {
				return true
			}
		}
	}

	return false
}

// HandleDirectActionKey processes a direct action key and performs the corresponding action.
// Returns the action name (e.g., "left_click", "move_mouse_relative"), whether the key was
// handled as a direct action, and any error if the action failed to execute.
func (s *ActionService) HandleDirectActionKey(
	ctx context.Context,
	key string,
) (string, bool, error) {
	actionString, logMsg, resolvedKey, ok := s.getActionMapping(key)

	if !ok {
		return "", false, nil
	}

	resolvedKeyLower := strings.ToLower(resolvedKey)

	if actionString == string(action.NameMoveMouseRelative) {
		var deltaX, deltaY int

		switch resolvedKeyLower {
		case strings.ToLower(s.keyBindings.MoveMouseUp):
			deltaY = -s.moveMouseStep
		case strings.ToLower(s.keyBindings.MoveMouseDown):
			deltaY = s.moveMouseStep
		case strings.ToLower(s.keyBindings.MoveMouseLeft):
			deltaX = -s.moveMouseStep
		case strings.ToLower(s.keyBindings.MoveMouseRight):
			deltaX = s.moveMouseStep
		default:
			return "", false, nil
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

			return actionString, true, moveErr
		}

		return actionString, true, nil
	}

	cursorPos, cursorPosErr := s.CursorPosition(ctx)
	if cursorPosErr != nil {
		s.logger.Error("Failed to get cursor position", zap.Error(cursorPosErr))

		return actionString, true, core.WrapAccessibilityFailed(cursorPosErr, "get cursor position")
	}

	s.logger.Info("Performing direct action",
		zap.String("action", logMsg),
		zap.Int("x", cursorPos.X),
		zap.Int("y", cursorPos.Y))

	performActionErr := s.PerformActionAtPoint(ctx, actionString, cursorPos)
	if performActionErr != nil {
		s.logger.Error("Failed to perform direct action", zap.Error(performActionErr))

		return actionString, true, performActionErr
	}

	return actionString, true, nil
}

// getActionMapping returns the action string, log message, and the resolved binding key for an action key.
// The resolved binding key is the key string that actually matched a configured binding
// (which may differ from the input key due to Shift+Letter normalization).
func (s *ActionService) getActionMapping(key string) (string, string, string, bool) {
	if key == "" {
		return "", "", "", false
	}

	normalizedKey := key
	// Normalize carriage return to "Return"
	if key == "\r" {
		normalizedKey = "Return"
	}

	// Check direct match first
	if s.matchActionKey(normalizedKey) {
		actionStr, logMsg, ok := s.getActionForBinding(normalizedKey)

		return actionStr, logMsg, normalizedKey, ok
	}

	// If key is a single uppercase letter (A-Z), check if Shift+Key is configured
	// This handles the case where Shift+Letter is pressed but C code sends uppercase letter
	if len(normalizedKey) == 1 {
		r := rune(normalizedKey[0])
		if r >= 'A' && r <= 'Z' {
			shiftKey := "Shift+" + normalizedKey
			if s.matchActionKey(shiftKey) {
				actionStr, logMsg, ok := s.getActionForBinding(shiftKey)

				return actionStr, logMsg, shiftKey, ok
			}
		}
	}

	return "", "", "", false
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
		action action.Name
		logMsg string
	}{
		{s.keyBindings.LeftClick, action.NameLeftClick, "Left click"},
		{s.keyBindings.RightClick, action.NameRightClick, "Right click"},
		{s.keyBindings.MiddleClick, action.NameMiddleClick, "Middle click"},
		{s.keyBindings.MouseDown, action.NameMouseDown, "Mouse down"},
		{s.keyBindings.MouseUp, action.NameMouseUp, "Mouse up"},
		{s.keyBindings.MoveMouseUp, action.NameMoveMouseRelative, "Move mouse up"},
		{s.keyBindings.MoveMouseDown, action.NameMoveMouseRelative, "Move mouse down"},
		{s.keyBindings.MoveMouseLeft, action.NameMoveMouseRelative, "Move mouse left"},
		{s.keyBindings.MoveMouseRight, action.NameMoveMouseRelative, "Move mouse right"},
	}

	for _, b := range bindings {
		if bindingLower == strings.ToLower(b.config) {
			return string(b.action), b.logMsg, true
		}
	}

	return "", "", false
}
