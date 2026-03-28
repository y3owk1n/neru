package services

import (
	"context"
	"image"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// ActionService handles executing actions on UI elements.
type ActionService struct {
	BaseService

	logger *zap.Logger
}

// NewActionService creates a new action service.
func NewActionService(
	accessibility ports.AccessibilityPort,
	overlay ports.OverlayPort,
	system ports.SystemPort,
	logger *zap.Logger,
) *ActionService {
	return &ActionService{
		BaseService: NewBaseService(accessibility, overlay, system),
		logger:      logger,
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
	modifiers action.Modifiers,
) error {
	// Parse action string to domain type
	actionType, actionTypeErr := action.ParseType(actionString)
	if actionTypeErr != nil {
		return core.WrapConfigFailed(actionTypeErr, "validate action type")
	}

	s.logger.Info("Performing action at point",
		zap.String("action", actionType.String()),
		zap.Int("x", point.X),
		zap.Int("y", point.Y),
		zap.String("modifiers", modifiers.String()))

	performActionErr := s.accessibility.PerformActionAtPoint(ctx, actionType, point, modifiers)
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

// MoveCursorToPointAndWait moves the cursor to the specified point and waits
// until any in-flight cursor animation settles before returning.
func (s *ActionService) MoveCursorToPointAndWait(
	ctx context.Context,
	point image.Point,
	bypassSmooth ...bool,
) error {
	shouldBypass := len(bypassSmooth) > 0 && bypassSmooth[0]

	err := s.system.MoveCursorToPoint(ctx, point, shouldBypass)
	if err != nil {
		return err
	}

	return s.system.WaitForCursorIdle(ctx)
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

// MoveMouseToCenterOfMonitor moves the mouse cursor to the center of the named
// monitor, optionally offset by the given delta values. The monitor name is
// matched case-insensitively against the localized display names reported by
// the operating system (e.g. "Built-in Retina Display", "DELL U2720Q").
func (s *ActionService) MoveMouseToCenterOfMonitor(
	ctx context.Context,
	monitorName string,
	offsetX, offsetY int,
) error {
	bounds, found, err := s.system.ScreenBoundsByName(ctx, monitorName)
	if err != nil {
		s.logger.Error("Failed to get screen bounds by name", zap.Error(err))

		return core.WrapAccessibilityFailed(err, "get screen bounds by name")
	}

	if !found {
		// Fetch available names to include in the error message for discoverability
		available := ""

		names, namesErr := s.system.ScreenNames(ctx)
		if namesErr == nil && len(names) > 0 {
			available = "; available monitors: " + strings.Join(names, ", ")
		}

		s.logger.Error("Monitor not found",
			zap.String("monitor", monitorName),
			zap.Strings("available", names))

		return derrors.Newf(
			derrors.CodeInvalidInput,
			"monitor not found: %s%s",
			monitorName,
			available,
		)
	}

	centerX := bounds.Min.X + bounds.Dx()/2 //nolint:mnd
	centerY := bounds.Min.Y + bounds.Dy()/2 //nolint:mnd
	target := image.Point{X: centerX + offsetX, Y: centerY + offsetY}

	return s.moveMouseWithBounds(
		ctx,
		target,
		bounds,
		false,
		zap.String("monitor", monitorName),
		zap.Int("offsetX", offsetX),
		zap.Int("offsetY", offsetY),
	)
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

	err := s.system.MoveCursorToPoint(ctx, clamped, bypassSmooth)
	if err != nil {
		return err
	}

	return s.system.WaitForCursorIdle(ctx)
}
