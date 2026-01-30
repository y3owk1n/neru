package mocks

import (
	"context"

	"github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// MockOverlayPort is a mock implementation of ports.OverlayPort.
type MockOverlayPort struct {
	ShowFunc      func()
	ShowHintsFunc func(context.Context, []*hint.Interface) error
	ShowGridFunc  func(ctx context.Context) error
	// DrawScrollIndicatorFunc mocks DrawScrollIndicator.
	DrawScrollIndicatorFunc func(x, y int)
	HideFunc                func(context.Context) error
	IsVisibleFunc           func() bool
	RefreshFunc             func(context.Context) error
	HealthFunc              func(context.Context) error

	// State tracking for tests
	visible bool
}

// Show implements ports.OverlayPort.
func (m *MockOverlayPort) Show() {
	if m.ShowFunc != nil {
		m.ShowFunc()
	}

	m.visible = true
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

// DrawScrollIndicator implements ports.OverlayPort.
func (m *MockOverlayPort) DrawScrollIndicator(x, y int) {
	if m.DrawScrollIndicatorFunc != nil {
		m.DrawScrollIndicatorFunc(x, y)
	}
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
