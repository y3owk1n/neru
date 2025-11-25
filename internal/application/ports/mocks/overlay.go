package mocks

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/domain/hint"
)

// MockOverlayPort is a mock implementation of ports.OverlayPort.
type MockOverlayPort struct {
	ShowHintsFunc func(context.Context, []*hint.Interface) error
	ShowGridFunc  func(context context.Context, rows, cols int) error
	// DrawScrollHighlightFunc mocks DrawScrollHighlight.
	DrawScrollHighlightFunc func(context context.Context, rect image.Rectangle, color string, width int) error
	DrawActionHighlightFunc func(context.Context, image.Rectangle, string, int) error
	HideFunc                func(context.Context) error
	IsVisibleFunc           func() bool
	RefreshFunc             func(context.Context) error
	HealthFunc              func(context.Context) error

	// State tracking for tests
	visible bool
}

// ShowHints implements ports.OverlayPort.
func (m *MockOverlayPort) ShowHints(context context.Context, hints []*hint.Interface) error {
	if m.ShowHintsFunc != nil {
		return m.ShowHintsFunc(context, hints)
	}

	m.visible = true

	return nil
}

// ShowGrid implements ports.OverlayPort.
func (m *MockOverlayPort) ShowGrid(context context.Context, rows, cols int) error {
	if m.ShowGridFunc != nil {
		return m.ShowGridFunc(context, rows, cols)
	}

	return nil
}

// DrawScrollHighlight implements ports.OverlayPort.
func (m *MockOverlayPort) DrawScrollHighlight(
	context context.Context,
	rect image.Rectangle,
	color string,
	width int,
) error {
	if m.DrawScrollHighlightFunc != nil {
		return m.DrawScrollHighlightFunc(context, rect, color, width)
	}

	return nil
}

// DrawActionHighlight implements ports.OverlayPort.
func (m *MockOverlayPort) DrawActionHighlight(
	context context.Context,
	rect image.Rectangle,
	color string,
	width int,
) error {
	if m.DrawActionHighlightFunc != nil {
		return m.DrawActionHighlightFunc(context, rect, color, width)
	}

	return nil
}

// Hide implements ports.OverlayPort.
func (m *MockOverlayPort) Hide(context context.Context) error {
	if m.HideFunc != nil {
		return m.HideFunc(context)
	}

	m.visible = false

	return nil
}

// IsVisible implements ports.OverlayPort.
func (m *MockOverlayPort) IsVisible() bool {
	if m.IsVisibleFunc != nil {
		return m.IsVisibleFunc()
	}

	return m.visible
}

// Refresh implements ports.OverlayPort.
func (m *MockOverlayPort) Refresh(context context.Context) error {
	if m.RefreshFunc != nil {
		return m.RefreshFunc(context)
	}

	return nil
}

// Health checks if the overlay manager is responsive.
func (m *MockOverlayPort) Health(context context.Context) error {
	if m.HealthFunc != nil {
		return m.HealthFunc(context)
	}

	return nil
}

// Ensure MockOverlayPort implements ports.OverlayPort.
var _ ports.OverlayPort = (*MockOverlayPort)(nil)
