package mocks

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/ports"
)

// MockMousePort is a mock implementation of ports.MousePort.
type MockMousePort struct {
	PerformActionAtPointFunc func(context.Context, action.Type, image.Point, action.Modifiers) error
	ScrollFunc               func(context.Context, int, int, action.Modifiers) error
	ReleaseHeldButtonsFunc   func(context.Context) error
}

// PerformActionAtPoint implements ports.MousePort.
func (m *MockMousePort) PerformActionAtPoint(
	ctx context.Context,
	actionType action.Type,
	point image.Point,
	modifiers action.Modifiers,
) error {
	if m.PerformActionAtPointFunc != nil {
		return m.PerformActionAtPointFunc(ctx, actionType, point, modifiers)
	}

	return nil
}

// Scroll implements ports.MousePort.
func (m *MockMousePort) Scroll(
	ctx context.Context,
	deltaX, deltaY int,
	modifiers action.Modifiers,
) error {
	if m.ScrollFunc != nil {
		return m.ScrollFunc(ctx, deltaX, deltaY, modifiers)
	}

	return nil
}

// ReleaseHeldButtons implements ports.MousePort.
func (m *MockMousePort) ReleaseHeldButtons(ctx context.Context) error {
	if m.ReleaseHeldButtonsFunc != nil {
		return m.ReleaseHeldButtonsFunc(ctx)
	}

	return nil
}

var _ ports.MousePort = (*MockMousePort)(nil)
