package mocks

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// MockOverlayPort is a mock implementation of ports.OverlayPort.
type MockOverlayPort struct {
	ShowHintsFunc func(context.Context, []*hint.Interface) error
	ShowGridFunc  func(ctx context.Context) error
	// DrawScrollHighlightFunc mocks DrawScrollHighlight.
	DrawScrollHighlightFunc func(ctx context.Context, rect image.Rectangle, color string, width int) error
	DrawActionHighlightFunc func(context.Context, image.Rectangle, string, int) error
	HideFunc                func(context.Context) error
	IsVisibleFunc           func() bool
	RefreshFunc             func(context.Context) error
	HealthFunc              func(context.Context) error

	// State tracking for tests
	visible bool
}

// ShowHints implements ports.OverlayPort.
func (m *MockOverlayPort) ShowHints(ctx context.Context, hints []*hint.Interface) error {
	if m.ShowHintsFunc != nil {
		return m.ShowHintsFunc(ctx, hints)
	}

	m.visible = true

	return nil
}

// ShowGrid implements ports.OverlayPort.
func (m *MockOverlayPort) ShowGrid(ctx context.Context) error {
	if m.ShowGridFunc != nil {
		return m.ShowGridFunc(ctx)
	}

	return nil
}

// DrawScrollHighlight implements ports.OverlayPort.
func (m *MockOverlayPort) DrawScrollHighlight(
	ctx context.Context,
	rect image.Rectangle,
	color string,
	width int,
) error {
	if m.DrawScrollHighlightFunc != nil {
		return m.DrawScrollHighlightFunc(ctx, rect, color, width)
	}

	return nil
}

// DrawActionHighlight implements ports.OverlayPort.
func (m *MockOverlayPort) DrawActionHighlight(
	ctx context.Context,
	rect image.Rectangle,
	color string,
	width int,
) error {
	if m.DrawActionHighlightFunc != nil {
		return m.DrawActionHighlightFunc(ctx, rect, color, width)
	}

	return nil
}

// Hide implements ports.OverlayPort.
func (m *MockOverlayPort) Hide(ctx context.Context) error {
	if m.HideFunc != nil {
		return m.HideFunc(ctx)
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
func (m *MockOverlayPort) Refresh(ctx context.Context) error {
	if m.RefreshFunc != nil {
		return m.RefreshFunc(ctx)
	}

	return nil
}

// Health checks if the overlay manager is responsive.
func (m *MockOverlayPort) Health(ctx context.Context) error {
	if m.HealthFunc != nil {
		return m.HealthFunc(ctx)
	}

	return nil
}

// Ensure MockOverlayPort implements ports.OverlayPort.
var _ ports.OverlayPort = (*MockOverlayPort)(nil)
