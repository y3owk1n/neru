package services

import (
	"context"
	"image"
	"slices"
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// ActionService handles executing actions on UI elements.
type ActionService struct {
	BaseService

	mu            sync.RWMutex
	config        config.ActionConfig
	keyBindings   config.ActionKeyBindingsCfg
	moveMouseStep int
	logger        *zap.Logger
}

// NewActionService creates a new action service.
func NewActionService(
	accessibility ports.AccessibilityPort,
	overlay ports.OverlayPort,
	system ports.SystemPort,
	actionConfig config.ActionConfig,
	keyBindings config.ActionKeyBindingsCfg,
	moveMouseStep int,
	logger *zap.Logger,
) *ActionService {
	return &ActionService{
		BaseService:   NewBaseService(accessibility, overlay, system),
		config:        actionConfig,
		keyBindings:   keyBindings,
		moveMouseStep: moveMouseStep,
		logger:        logger,
	}
}

// UpdateConfig updates the action service configuration.
// This allows changing action key bindings and move mouse step at runtime.
func (s *ActionService) UpdateConfig(actionConfig config.ActionConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = actionConfig
	s.keyBindings = actionConfig.KeyBindings
	s.moveMouseStep = actionConfig.MoveMouseStep

	s.logger.Info("Action configuration updated",
		zap.String("left_click", actionConfig.KeyBindings.LeftClick),
		zap.String("right_click", actionConfig.KeyBindings.RightClick),
		zap.Int("move_mouse_step", actionConfig.MoveMouseStep))
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

	return s.system.MoveCursorToPoint(ctx, center, false)
}

// MoveCursorToPoint moves the cursor to the specified point.
func (s *ActionService) MoveCursorToPoint(ctx context.Context, point image.Point) error {
	return s.system.MoveCursorToPoint(ctx, point, false)
}

// clampToScreenBounds clamps the given point so it stays within the screen bounds.
func clampToScreenBounds(point image.Point, bounds image.Rectangle) image.Point {
	maxX := max(bounds.Max.X-1, bounds.Min.X)
	maxY := max(bounds.Max.Y-1, bounds.Min.Y)

	return image.Point{
		X: min(max(point.X, bounds.Min.X), maxX),
		Y: min(max(point.Y, bounds.Min.Y), maxY),
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
	screenBounds, err := s.system.ScreenBounds(ctx)
	if err != nil {
		s.logger.Error("Failed to get screen bounds", zap.Error(err))

		return core.WrapAccessibilityFailed(err, "get screen bounds")
	}

	shouldBypass := len(bypassSmooth) > 0 && bypassSmooth[0]

	return s.moveMouseWithBounds(
		ctx,
		image.Point{X: targetX, Y: targetY},
		screenBounds,
		shouldBypass,
	)
}

// MoveMouseRelative moves the mouse cursor by the specified delta from the current position.
// If bypassSmooth is true, smooth cursor animation is skipped (used for keyboard-driven movements).
func (s *ActionService) MoveMouseRelative(
	ctx context.Context,
	deltaX, deltaY int,
	bypassSmooth ...bool,
) error {
	cursorPos, err := s.system.CursorPosition(ctx)
	if err != nil {
		s.logger.Error("Failed to get cursor position", zap.Error(err))

		return core.WrapAccessibilityFailed(err, "get cursor position")
	}

	shouldBypass := len(bypassSmooth) > 0 && bypassSmooth[0]

	return s.MoveMouseTo(ctx, cursorPos.X+deltaX, cursorPos.Y+deltaY, shouldBypass)
}

// CursorPosition returns the current cursor position.
func (s *ActionService) CursorPosition(ctx context.Context) (image.Point, error) {
	return s.system.CursorPosition(ctx)
}

// ScreenBounds returns the bounds of the active screen.
func (s *ActionService) ScreenBounds(ctx context.Context) (image.Rectangle, error) {
	return s.system.ScreenBounds(ctx)
}

// MoveMouseToCenter moves the mouse cursor to the center of the active screen,
// optionally offset by the given delta values. It fetches screen bounds once
// for both center computation and clamping.
func (s *ActionService) MoveMouseToCenter(ctx context.Context, offsetX, offsetY int) error {
	screenBounds, err := s.system.ScreenBounds(ctx)
	if err != nil {
		s.logger.Error("Failed to get screen bounds", zap.Error(err))

		return core.WrapAccessibilityFailed(err, "get screen bounds")
	}

	centerX := screenBounds.Min.X + screenBounds.Dx()/2 //nolint:mnd
	centerY := screenBounds.Min.Y + screenBounds.Dy()/2 //nolint:mnd
	target := image.Point{X: centerX + offsetX, Y: centerY + offsetY}

	return s.moveMouseWithBounds(
		ctx,
		target,
		screenBounds,
		false,
		zap.Int("offsetX", offsetX),
		zap.Int("offsetY", offsetY),
	)
}

// HandleActionKey processes an action key and performs the corresponding action at the current cursor position.
// Returns true if the key was handled as an action key, false otherwise.
// Returns an error if the action failed to execute.
func (s *ActionService) HandleActionKey(
	ctx context.Context,
	key string,
	mode string,
) (bool, error) {
	s.mu.RLock()
	act, logMsg, _, ok := s.getActionMapping(key)
	s.mu.RUnlock()

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
// Uses NormalizeKeyForComparison for consistent matching, and handles
// Shift+Letter normalization (e.g. "K" matching "Shift+K").
func (s *ActionService) IsMoveMouseKey(key string) bool {
	if key == "" {
		return false
	}

	s.mu.RLock()
	bindings := []string{
		s.keyBindings.MoveMouseUp,
		s.keyBindings.MoveMouseDown,
		s.keyBindings.MoveMouseLeft,
		s.keyBindings.MoveMouseRight,
	}
	s.mu.RUnlock()

	normalized := config.NormalizeKeyForComparison(key)
	keysToCheck := []string{normalized}
	// If the ORIGINAL key is a single uppercase letter, also check against Shift+Key.
	// Must use original key to avoid matching plain lowercase letters against Shift bindings.
	if len(key) == 1 {
		r := rune(key[0])
		if r >= 'A' && r <= 'Z' {
			keysToCheck = append(keysToCheck, config.NormalizeKeyForComparison("Shift+"+key))
		}
	}

	for _, binding := range bindings {
		if binding == "" {
			continue
		}

		normalizedBinding := config.NormalizeKeyForComparison(binding)

		if slices.Contains(keysToCheck, normalizedBinding) {
			return true
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
	s.mu.RLock()
	actionString, logMsg, resolvedKey, ok := s.getActionMapping(key)
	keyBindings := s.keyBindings
	step := s.moveMouseStep
	s.mu.RUnlock()

	if !ok {
		return "", false, nil
	}

	// resolvedKey is already normalized by getActionMapping, so we only
	// need to normalize the config binding values for comparison.
	if actionString == string(action.NameMoveMouseRelative) {
		var deltaX, deltaY int

		switch resolvedKey {
		case config.NormalizeKeyForComparison(keyBindings.MoveMouseUp):
			deltaY = -step
		case config.NormalizeKeyForComparison(keyBindings.MoveMouseDown):
			deltaY = step
		case config.NormalizeKeyForComparison(keyBindings.MoveMouseLeft):
			deltaX = -step
		case config.NormalizeKeyForComparison(keyBindings.MoveMouseRight):
			deltaX = step
		default:
			return "", false, nil
		}

		s.logger.Info("Performing move mouse relative",
			zap.String("action", logMsg),
			zap.Int("dx", deltaX),
			zap.Int("dy", deltaY),
			zap.Int("step", step),
		)

		moveErr := s.MoveMouseRelative(ctx, deltaX, deltaY, true)
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

// moveMouseWithBounds clamps target to the given screen bounds and moves the cursor.
// This is the shared implementation used by MoveMouseTo and MoveMouseToCenter to
// avoid duplicating clamp-and-move logic.
func (s *ActionService) moveMouseWithBounds(
	ctx context.Context,
	target image.Point,
	screenBounds image.Rectangle,
	bypassSmooth bool,
	fields ...zap.Field,
) error {
	clamped := clampToScreenBounds(target, screenBounds)
	baseCount := 4
	logFields := make([]zap.Field, 0, baseCount+len(fields))
	logFields = append(logFields,
		zap.Int("x", clamped.X),
		zap.Int("y", clamped.Y),
		zap.Bool("clamped", clamped != target),
		zap.Bool("bypassSmooth", bypassSmooth),
	)
	logFields = append(logFields, fields...)
	s.logger.Info("Moving mouse cursor", logFields...)

	return s.system.MoveCursorToPoint(ctx, clamped, bypassSmooth)
}

// getActionMapping returns the action string, log message, and the resolved binding key for an action key.
// The resolved binding key is the key string that actually matched a configured binding
// (which may differ from the input key due to Shift+Letter normalization).
func (s *ActionService) getActionMapping(key string) (string, string, string, bool) {
	if key == "" {
		return "", "", "", false
	}

	// Normalize special key representations to their canonical forms
	// so that "\r" matches "Return", "\x1b" matches "Escape", etc.
	normalizedKey := config.NormalizeKeyForComparison(key)

	// Check direct match first
	if s.matchActionKey(normalizedKey) {
		actionStr, logMsg, ok := s.getActionForBinding(normalizedKey)

		return actionStr, logMsg, normalizedKey, ok
	}

	// If the ORIGINAL key is a single uppercase letter (A-Z), check if Shift+Key is configured.
	// This handles the case where Shift+Letter is pressed but the event tap sends just
	// the uppercase letter (e.g. "L" instead of "Shift+L") when keyCodeToName returns nil.
	// We must check the original key (not the normalized one) because normalization lowercases,
	// which would cause plain lowercase "l" to falsely trigger the Shift+L binding.
	if len(key) == 1 {
		r := rune(key[0])
		if r >= 'A' && r <= 'Z' {
			shiftKey := config.NormalizeKeyForComparison("Shift+" + key)
			if s.matchActionKey(shiftKey) {
				actionStr, logMsg, ok := s.getActionForBinding(shiftKey)

				return actionStr, logMsg, shiftKey, ok
			}
		}
	}

	return "", "", "", false
}

// matchActionKey checks if a pre-normalized key matches any configured binding.
// The key parameter must already be normalized via NormalizeKeyForComparison;
// only the binding values (from config) are normalized here.
func (s *ActionService) matchActionKey(normalizedKey string) bool {
	return normalizedKey == config.NormalizeKeyForComparison(s.keyBindings.LeftClick) ||
		normalizedKey == config.NormalizeKeyForComparison(s.keyBindings.RightClick) ||
		normalizedKey == config.NormalizeKeyForComparison(s.keyBindings.MiddleClick) ||
		normalizedKey == config.NormalizeKeyForComparison(s.keyBindings.MouseDown) ||
		normalizedKey == config.NormalizeKeyForComparison(s.keyBindings.MouseUp) ||
		normalizedKey == config.NormalizeKeyForComparison(s.keyBindings.MoveMouseUp) ||
		normalizedKey == config.NormalizeKeyForComparison(s.keyBindings.MoveMouseDown) ||
		normalizedKey == config.NormalizeKeyForComparison(s.keyBindings.MoveMouseLeft) ||
		normalizedKey == config.NormalizeKeyForComparison(s.keyBindings.MoveMouseRight)
}

// getActionForBinding returns the action for a pre-normalized binding key.
// The binding parameter must already be normalized via NormalizeKeyForComparison;
// only the config values are normalized here.
func (s *ActionService) getActionForBinding(normalizedBinding string) (string, string, bool) {
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
		if normalizedBinding == config.NormalizeKeyForComparison(b.config) {
			return string(b.action), b.logMsg, true
		}
	}

	return "", "", false
}
