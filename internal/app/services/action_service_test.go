package services_test

import (
	"context"
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	portmocks "github.com/y3owk1n/neru/internal/core/ports/mocks"
)

func newTestActionService(
	acc *portmocks.MockAccessibilityPort,
	sys *portmocks.SystemMock,
) *services.ActionService {
	return services.NewActionService(acc, &portmocks.MockOverlayPort{}, sys, zap.NewNop())
}

func TestPerformActionAtPoint_ParsesAndDispatches(t *testing.T) {
	ctx := context.Background()

	called := false
	acc := &portmocks.MockAccessibilityPort{
		PerformActionAtPointFunc: func(
			_ context.Context,
			actionType action.Type,
			point image.Point,
			_ action.Modifiers,
		) error {
			called = true

			if actionType != action.TypeLeftClick {
				t.Fatalf("unexpected action type: %v", actionType)
			}

			if point != (image.Point{X: 10, Y: 20}) {
				t.Fatalf("unexpected point: %v", point)
			}

			return nil
		},
	}
	service := newTestActionService(acc, &portmocks.SystemMock{})

	err := service.PerformActionAtPoint(
		ctx,
		"left_click",
		image.Point{X: 10, Y: 20},
		0,
	)
	if err != nil {
		t.Fatalf("PerformActionAtPoint() error = %v", err)
	}

	if !called {
		t.Fatal("expected PerformActionAtPoint to be called")
	}
}

func TestPerformActionAtPoint_InvalidAction(t *testing.T) {
	service := newTestActionService(&portmocks.MockAccessibilityPort{}, &portmocks.SystemMock{})

	err := service.PerformActionAtPoint(context.Background(), "not_real", image.Point{}, 0)
	if err == nil {
		t.Fatal("expected error for invalid action string")
	}
}

func TestMoveMouseTo_ClampsToScreenBounds(t *testing.T) {
	ctx := context.Background()

	var moved image.Point

	waitCalled := false

	sys := &portmocks.SystemMock{
		ScreenBoundsFunc: func(context.Context) (image.Rectangle, error) {
			return image.Rect(0, 0, 100, 100), nil
		},
		MoveCursorToPointFunc: func(_ context.Context, p image.Point, _ bool) error {
			moved = p

			return nil
		},
		WaitForCursorIdleFunc: func(context.Context) error {
			waitCalled = true

			return nil
		},
	}
	service := newTestActionService(&portmocks.MockAccessibilityPort{}, sys)

	err := service.MoveMouseTo(ctx, 1000, -5)
	if err != nil {
		t.Fatalf("MoveMouseTo() error = %v", err)
	}

	if moved != (image.Point{X: 99, Y: 0}) {
		t.Fatalf("MoveMouseTo() moved to %v, want (99,0)", moved)
	}

	if !waitCalled {
		t.Fatal("MoveMouseTo() expected WaitForCursorIdle to be called")
	}
}

func TestMoveMouseRelative_UsesCurrentCursorPosition(t *testing.T) {
	ctx := context.Background()

	var moved image.Point

	sys := &portmocks.SystemMock{
		CursorPositionFunc: func(context.Context) (image.Point, error) {
			return image.Point{X: 40, Y: 40}, nil
		},
		ScreenBoundsFunc: func(context.Context) (image.Rectangle, error) {
			return image.Rect(0, 0, 100, 100), nil
		},
		MoveCursorToPointFunc: func(_ context.Context, p image.Point, _ bool) error {
			moved = p

			return nil
		},
	}
	service := newTestActionService(&portmocks.MockAccessibilityPort{}, sys)

	err := service.MoveMouseRelative(ctx, 10, -5)
	if err != nil {
		t.Fatalf("MoveMouseRelative() error = %v", err)
	}

	if moved != (image.Point{X: 50, Y: 35}) {
		t.Fatalf("MoveMouseRelative() moved to %v, want (50,35)", moved)
	}
}

func TestMoveCursorToPointAndWait_WaitsForCursorIdle(t *testing.T) {
	ctx := context.Background()

	moved := false
	waitCalled := false

	sys := &portmocks.SystemMock{
		MoveCursorToPointFunc: func(_ context.Context, p image.Point, _ bool) error {
			moved = true

			if p != (image.Point{X: 12, Y: 34}) {
				t.Fatalf("MoveCursorToPointAndWait() point = %v, want (12,34)", p)
			}

			if waitCalled {
				t.Fatal("WaitForCursorIdle called before MoveCursorToPoint completed")
			}

			return nil
		},
		WaitForCursorIdleFunc: func(context.Context) error {
			waitCalled = true

			if !moved {
				t.Fatal("WaitForCursorIdle called before MoveCursorToPoint")
			}

			return nil
		},
	}

	service := newTestActionService(&portmocks.MockAccessibilityPort{}, sys)

	err := service.MoveCursorToPointAndWait(ctx, image.Point{X: 12, Y: 34})
	if err != nil {
		t.Fatalf("MoveCursorToPointAndWait() error = %v", err)
	}

	if !waitCalled {
		t.Fatal("MoveCursorToPointAndWait() expected WaitForCursorIdle to be called")
	}
}
