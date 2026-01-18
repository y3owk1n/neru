package mocks

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// MockAccessibilityPort is a mock implementation of ports.AccessibilityPort.
type MockAccessibilityPort struct {
	HealthFunc               func(context.Context) error
	ClickableElementsFunc    func(context.Context, ports.ElementFilter) ([]*element.Element, error)
	PerformActionFunc        func(context.Context, *element.Element, action.Type) error
	PerformActionAtPointFunc func(context.Context, action.Type, image.Point) error
	ScrollFunc               func(context.Context, int, int) error
	FocusedAppBundleIDFunc   func(context.Context) (string, error)
	IsAppExcludedFunc        func(context.Context, string) bool
	ScreenBoundsFunc         func(context.Context) (image.Rectangle, error)
	MoveCursorToPointFunc    func(context.Context, image.Point, bool) error
	CursorPositionFunc       func(context.Context) (image.Point, error)
	CheckPermissionsFunc     func(context.Context) error
}

// Health implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) Health(ctx context.Context) error {
	if m.HealthFunc != nil {
		return m.HealthFunc(ctx)
	}

	return nil
}

// ClickableElements implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) ClickableElements(
	ctx context.Context,
	filter ports.ElementFilter,
) ([]*element.Element, error) {
	if m.ClickableElementsFunc != nil {
		return m.ClickableElementsFunc(ctx, filter)
	}

	return nil, nil
}

// PerformAction implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) PerformAction(
	ctx context.Context,
	elem *element.Element,
	actionType action.Type,
) error {
	if m.PerformActionFunc != nil {
		return m.PerformActionFunc(ctx, elem, actionType)
	}

	return nil
}

// PerformActionAtPoint implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) PerformActionAtPoint(
	ctx context.Context,
	actionType action.Type,
	point image.Point,
) error {
	if m.PerformActionAtPointFunc != nil {
		return m.PerformActionAtPointFunc(ctx, actionType, point)
	}

	return nil
}

// Scroll implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) Scroll(ctx context.Context, deltaX, deltaY int) error {
	if m.ScrollFunc != nil {
		return m.ScrollFunc(ctx, deltaX, deltaY)
	}

	return nil
}

// FocusedAppBundleID implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) FocusedAppBundleID(ctx context.Context) (string, error) {
	if m.FocusedAppBundleIDFunc != nil {
		return m.FocusedAppBundleIDFunc(ctx)
	}

	return "", nil
}

// IsAppExcluded implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) IsAppExcluded(ctx context.Context, bundleID string) bool {
	if m.IsAppExcludedFunc != nil {
		return m.IsAppExcludedFunc(ctx, bundleID)
	}

	return false
}

// ScreenBounds implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) ScreenBounds(ctx context.Context) (image.Rectangle, error) {
	if m.ScreenBoundsFunc != nil {
		return m.ScreenBoundsFunc(ctx)
	}

	return image.Rectangle{}, nil
}

// MoveCursorToPoint implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) MoveCursorToPoint(
	ctx context.Context,
	point image.Point,
	bypassSmooth bool,
) error {
	if m.MoveCursorToPointFunc != nil {
		return m.MoveCursorToPointFunc(ctx, point, bypassSmooth)
	}

	return nil
}

// CursorPosition implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) CursorPosition(ctx context.Context) (image.Point, error) {
	if m.CursorPositionFunc != nil {
		return m.CursorPositionFunc(ctx)
	}

	return image.Point{}, nil
}

// CheckPermissions implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) CheckPermissions(ctx context.Context) error {
	if m.CheckPermissionsFunc != nil {
		return m.CheckPermissionsFunc(ctx)
	}

	return nil
}
