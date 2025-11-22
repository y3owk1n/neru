// Package mocks provides mock implementations of port interfaces for testing.
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
	ScrollFunc func(ctx context.Context, deltaX, deltaY int) error

	// MoveCursorToPointFunc mocks MoveCursorToPoint.
	MoveCursorToPointFunc func(ctx context.Context, point image.Point) error

	// GetCursorPositionFunc mocks GetCursorPosition.
	GetCursorPositionFunc func(ctx context.Context) (image.Point, error)
}

// GetClickableElements implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) GetClickableElements(
	ctx context.Context,
	filter ports.ElementFilter,
) ([]*element.Element, error) {
	if m.GetClickableElementsFunc != nil {
		return m.GetClickableElementsFunc(ctx, filter)
	}
	return nil, nil
}

// GetScrollableElements implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) GetScrollableElements(
	ctx context.Context,
) ([]*element.Element, error) {
	if m.GetScrollableElementsFunc != nil {
		return m.GetScrollableElementsFunc(ctx)
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

// GetFocusedAppBundleID implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) GetFocusedAppBundleID(ctx context.Context) (string, error) {
	if m.GetFocusedAppBundleIDFunc != nil {
		return m.GetFocusedAppBundleIDFunc(ctx)
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

// GetScreenBounds implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) GetScreenBounds(ctx context.Context) (image.Rectangle, error) {
	if m.GetScreenBoundsFunc != nil {
		return m.GetScreenBoundsFunc(ctx)
	}
	return image.Rectangle{}, nil
}

// CheckPermissions implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) CheckPermissions(ctx context.Context) error {
	if m.CheckPermissionsFunc != nil {
		return m.CheckPermissionsFunc(ctx)
	}
	return nil
}

// MoveCursorToPoint implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) MoveCursorToPoint(ctx context.Context, point image.Point) error {
	if m.MoveCursorToPointFunc != nil {
		return m.MoveCursorToPointFunc(ctx, point)
	}
	return nil
}

// GetCursorPosition implements ports.AccessibilityPort.
func (m *MockAccessibilityPort) GetCursorPosition(ctx context.Context) (image.Point, error) {
	if m.GetCursorPositionFunc != nil {
		return m.GetCursorPositionFunc(ctx)
	}
	return image.Point{}, nil
}

// Ensure MockAccessibilityPort implements ports.AccessibilityPort
var _ ports.AccessibilityPort = (*MockAccessibilityPort)(nil)
