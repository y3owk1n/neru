package mocks

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// MockAccessibilityPort is a mock implementation of ports.AccessibilityPort.
type MockAccessibilityPort struct {
	GetClickableElementsFunc  func(context.Context, ports.ElementFilter) ([]*element.Element, error)
	GetScrollableElementsFunc func(context.Context) ([]*element.Element, error)
	PerformActionFunc         func(context.Context, *element.Element, action.Type) error
	GetFocusedAppBundleIDFunc func(context.Context) (string, error)
	IsAppExcludedFunc         func(context.Context, string) bool
	GetScreenBoundsFunc       func(context.Context) (image.Rectangle, error)
	CheckPermissionsFunc      func(context.Context) error
	// PerformActionAtPointFunc mocks PerformActionAtPoint.
	PerformActionAtPointFunc func(ctx context.Context, actionType action.Type, point image.Point) error

	// ScrollFunc mocks Scroll.
	ScrollFunc func(context context.Context, deltaX, deltaY int) error

	// MoveCursorToPointFunc mocks MoveCursorToPoint.
	MoveCursorToPointFunc func(context context.Context, point image.Point) error

	// GetCursorPositionFunc mocks GetCursorPosition.
	GetCursorPositionFunc func(context context.Context) (image.Point, error)
	HealthFunc            func(context.Context) error
}

// GetClickableElements implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) GetClickableElements(
	context context.Context,
	filter ports.ElementFilter,
) ([]*element.Element, error) {
	if m.GetClickableElementsFunc != nil {
		return m.GetClickableElementsFunc(context, filter)
	}

	return nil, nil
}

// GetScrollableElements implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) GetScrollableElements(
	context context.Context,
) ([]*element.Element, error) {
	if m.GetScrollableElementsFunc != nil {
		return m.GetScrollableElementsFunc(context)
	}

	return nil, nil
}

// PerformAction implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) PerformAction(
	context context.Context,
	element *element.Element,
	actionType action.Type,
) error {
	if m.PerformActionFunc != nil {
		return m.PerformActionFunc(context, element, actionType)
	}

	return nil
}

// PerformActionAtPoint implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) PerformActionAtPoint(
	context context.Context,
	actionType action.Type,
	point image.Point,
) error {
	if m.PerformActionAtPointFunc != nil {
		return m.PerformActionAtPointFunc(context, actionType, point)
	}

	return nil
}

// Scroll implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) Scroll(context context.Context, deltaX, deltaY int) error {
	if m.ScrollFunc != nil {
		return m.ScrollFunc(context, deltaX, deltaY)
	}

	return nil
}

// GetFocusedAppBundleID implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) GetFocusedAppBundleID(context context.Context) (string, error) {
	if m.GetFocusedAppBundleIDFunc != nil {
		return m.GetFocusedAppBundleIDFunc(context)
	}

	return "", nil
}

// IsAppExcluded implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) IsAppExcluded(context context.Context, bundleID string) bool {
	if m.IsAppExcludedFunc != nil {
		return m.IsAppExcludedFunc(context, bundleID)
	}

	return false
}

// GetScreenBounds implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) GetScreenBounds(context context.Context) (image.Rectangle, error) {
	if m.GetScreenBoundsFunc != nil {
		return m.GetScreenBoundsFunc(context)
	}

	return image.Rectangle{}, nil
}

// CheckPermissions implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) CheckPermissions(context context.Context) error {
	if m.CheckPermissionsFunc != nil {
		return m.CheckPermissionsFunc(context)
	}

	return nil
}

// MoveCursorToPoint implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) MoveCursorToPoint(
	context context.Context,
	point image.Point,
) error {
	if m.MoveCursorToPointFunc != nil {
		return m.MoveCursorToPointFunc(context, point)
	}

	return nil
}

// GetCursorPosition implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) GetCursorPosition(context context.Context) (image.Point, error) {
	if m.GetCursorPositionFunc != nil {
		return m.GetCursorPositionFunc(context)
	}

	return image.Point{}, nil
}

// Health checks if the accessibility permissions are granted.
func (m *MockAccessibilityPort) Health(context context.Context) error {
	if m.HealthFunc != nil {
		return m.HealthFunc(context)
	}

	return nil
}

// Ensure MockAccessibilityPort implements ports.AccessibilityPort.
var _ ports.AccessibilityPort = (*MockAccessibilityPort)(nil)
