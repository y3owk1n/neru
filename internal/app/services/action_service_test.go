package services_test

import (
	"context"
	"image"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/ports"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

const leftClickAction = "left_click"

func newTestActionService(
	acc *portmocks.MockAccessibilityPort,
	sys ports.SystemPort,
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
	service := newTestActionService(acc, &portmocks.MockSystemPort{})

	err := service.PerformActionAtPoint(
		ctx,
		leftClickAction,
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

func TestPerformActionAtPoint_DrawsMouseActionIndicatorWhenEnabled(t *testing.T) {
	ctx := context.Background()

	acc := &portmocks.MockAccessibilityPort{
		PerformActionAtPointFunc: func(
			context.Context,
			action.Type,
			image.Point,
			action.Modifiers,
		) error {
			return nil
		},
	}

	var (
		gotPoint image.Point
		gotStyle ports.MouseActionIndicatorStyle
	)

	drawn := false
	overlay := &portmocks.MockOverlayPort{
		DrawMouseActionIndicatorFunc: func(
			point image.Point,
			style ports.MouseActionIndicatorStyle,
		) {
			drawn = true
			gotPoint = point
			gotStyle = style
		},
	}

	service := services.NewActionService(acc, overlay, &portmocks.MockSystemPort{}, zap.NewNop())
	cfg := config.DefaultConfig().MouseAction
	cfg.Enabled = true
	cfg.Actions = []string{leftClickAction}
	service.UpdateConfig(cfg)

	point := image.Point{X: 10, Y: 20}

	err := service.PerformActionAtPoint(ctx, leftClickAction, point, 0)
	if err != nil {
		t.Fatalf("PerformActionAtPoint() error = %v", err)
	}

	if !drawn {
		t.Fatal("expected mouse action indicator to be drawn")
	}

	if gotPoint != point {
		t.Fatalf("unexpected indicator point: %v", gotPoint)
	}

	if gotStyle.Size != cfg.UI.Size || gotStyle.DurationMS != cfg.Animation.DurationMS {
		t.Fatalf("unexpected indicator style: %+v", gotStyle)
	}
}

func TestPerformActionAtPoint_DoesNotDrawMouseActionIndicatorWhenDisabled(t *testing.T) {
	ctx := context.Background()

	acc := &portmocks.MockAccessibilityPort{
		PerformActionAtPointFunc: func(
			context.Context,
			action.Type,
			image.Point,
			action.Modifiers,
		) error {
			return nil
		},
	}

	drawn := false
	overlay := &portmocks.MockOverlayPort{
		DrawMouseActionIndicatorFunc: func(
			image.Point,
			ports.MouseActionIndicatorStyle,
		) {
			drawn = true
		},
	}

	service := services.NewActionService(acc, overlay, &portmocks.MockSystemPort{}, zap.NewNop())
	cfg := config.DefaultConfig().MouseAction
	cfg.Enabled = false
	cfg.Actions = []string{leftClickAction}
	service.UpdateConfig(cfg)

	err := service.PerformActionAtPoint(ctx, leftClickAction, image.Point{X: 10, Y: 20}, 0)
	if err != nil {
		t.Fatalf("PerformActionAtPoint() error = %v", err)
	}

	if drawn {
		t.Fatal("expected mouse action indicator NOT to be drawn when disabled")
	}
}

func TestPerformActionAtPoint_DoesNotDrawMouseActionIndicatorForUnlistedAction(t *testing.T) {
	ctx := context.Background()

	acc := &portmocks.MockAccessibilityPort{
		PerformActionAtPointFunc: func(
			context.Context,
			action.Type,
			image.Point,
			action.Modifiers,
		) error {
			return nil
		},
	}

	drawn := false
	overlay := &portmocks.MockOverlayPort{
		DrawMouseActionIndicatorFunc: func(
			image.Point,
			ports.MouseActionIndicatorStyle,
		) {
			drawn = true
		},
	}

	service := services.NewActionService(acc, overlay, &portmocks.MockSystemPort{}, zap.NewNop())
	cfg := config.DefaultConfig().MouseAction
	cfg.Enabled = true
	cfg.Actions = []string{leftClickAction}
	service.UpdateConfig(cfg)

	err := service.PerformActionAtPoint(ctx, "right_click", image.Point{X: 10, Y: 20}, 0)
	if err != nil {
		t.Fatalf("PerformActionAtPoint() error = %v", err)
	}

	if drawn {
		t.Fatal("expected mouse action indicator NOT to be drawn for unlisted action")
	}
}

func TestPerformActionAtPoint_InvalidAction(t *testing.T) {
	service := newTestActionService(&portmocks.MockAccessibilityPort{}, &portmocks.MockSystemPort{})

	err := service.PerformActionAtPoint(context.Background(), "not_real", image.Point{}, 0)
	if err == nil {
		t.Fatal("expected error for invalid action string")
	}
}

func TestMoveMouseTo_ClampsToScreenBounds(t *testing.T) {
	ctx := context.Background()

	var moved image.Point

	waitCalled := false

	sys := &portmocks.MockSystemPort{
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

	sys := &portmocks.MockSystemPort{
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

type relativeCursorSystem struct {
	*portmocks.MockSystemPort

	handled bool
	moved   image.Point
}

func (s *relativeCursorSystem) MoveCursorBy(
	_ context.Context,
	delta image.Point,
) (bool, error) {
	s.moved = delta

	return s.handled, nil
}

func TestMoveMouseRelative_UsesNativeRelativeMovement(t *testing.T) {
	ctx := context.Background()
	sys := &relativeCursorSystem{
		MockSystemPort: &portmocks.MockSystemPort{
			CursorPositionFunc: func(context.Context) (image.Point, error) {
				t.Fatal("CursorPosition() should not be called for native relative movement")

				return image.Point{}, nil
			},
		},
		handled: true,
	}
	service := newTestActionService(&portmocks.MockAccessibilityPort{}, sys)

	err := service.MoveMouseRelative(ctx, 10, -5)
	if err != nil {
		t.Fatalf("MoveMouseRelative() error = %v", err)
	}

	if sys.moved != (image.Point{X: 10, Y: -5}) {
		t.Fatalf("MoveMouseRelative() delta = %v, want (10,-5)", sys.moved)
	}
}

func TestMoveCursorToPointAndWait_WaitsForCursorIdle(t *testing.T) {
	ctx := context.Background()

	moved := false
	waitCalled := false

	sys := &portmocks.MockSystemPort{
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

// TestPerformActionAtPoint_MouseActionIndicatorMatchesDeprecatedName pins that a
// config written before the buttons were spelled out ("mouse_down") still
// matches the left press action, which now reports itself as left_mouse_down.
func TestPerformActionAtPoint_MouseActionIndicatorMatchesDeprecatedName(t *testing.T) {
	ctx := context.Background()

	acc := &portmocks.MockAccessibilityPort{
		PerformActionAtPointFunc: func(
			context.Context,
			action.Type,
			image.Point,
			action.Modifiers,
		) error {
			return nil
		},
	}

	drawn := false
	overlay := &portmocks.MockOverlayPort{
		DrawMouseActionIndicatorFunc: func(image.Point, ports.MouseActionIndicatorStyle) {
			drawn = true
		},
	}

	service := services.NewActionService(acc, overlay, &portmocks.MockSystemPort{}, zap.NewNop())
	cfg := config.DefaultConfig().MouseAction
	cfg.Enabled = true
	cfg.Actions = []string{"mouse_down"}
	service.UpdateConfig(cfg)

	err := service.PerformActionAtPoint(ctx, "left_mouse_down", image.Point{X: 5, Y: 6}, 0)
	if err != nil {
		t.Fatalf("PerformActionAtPoint() error = %v", err)
	}

	if !drawn {
		t.Fatal("expected mouse action indicator to be drawn for the deprecated config name")
	}
}

// TestPerformActionAtPoint_MouseActionIndicatorIgnoresUnparseableEntry pins that
// a typo in the config list does not match anything (and does not panic).
func TestPerformActionAtPoint_MouseActionIndicatorIgnoresUnparseableEntry(t *testing.T) {
	ctx := context.Background()

	acc := &portmocks.MockAccessibilityPort{
		PerformActionAtPointFunc: func(
			context.Context,
			action.Type,
			image.Point,
			action.Modifiers,
		) error {
			return nil
		},
	}

	drawn := false
	overlay := &portmocks.MockOverlayPort{
		DrawMouseActionIndicatorFunc: func(image.Point, ports.MouseActionIndicatorStyle) {
			drawn = true
		},
	}

	service := services.NewActionService(acc, overlay, &portmocks.MockSystemPort{}, zap.NewNop())
	cfg := config.DefaultConfig().MouseAction
	cfg.Enabled = true
	cfg.Actions = []string{"not_an_action"}
	service.UpdateConfig(cfg)

	err := service.PerformActionAtPoint(ctx, "right_mouse_toggle", image.Point{X: 5, Y: 6}, 0)
	if err != nil {
		t.Fatalf("PerformActionAtPoint() error = %v", err)
	}

	if drawn {
		t.Fatal("expected no indicator for an unrecognized config entry")
	}
}

// TestMoveMouseToCenterOfWindow_KeepsANotSupportedRefusalIntact pins what a
// person sees when their compositor cannot report focused-window geometry at
// all. The platform refuses with CodeNotSupported and names what is missing;
// re-coding that as an accessibility failure would send them to check a
// permission they have already granted.
func TestMoveMouseToCenterOfWindow_KeepsANotSupportedRefusalIntact(t *testing.T) {
	refusal := derrors.New(
		derrors.CodeNotSupported,
		"no focused-window geometry source on linux backend wayland-wlroots",
	)

	sys := &portmocks.MockSystemPort{
		FocusedWindowBoundsFunc: func(context.Context) (image.Rectangle, bool, error) {
			return image.Rectangle{}, false, refusal
		},
	}
	service := newTestActionService(&portmocks.MockAccessibilityPort{}, sys)

	err := service.MoveMouseToCenterOfWindow(context.Background(), 0, 0)
	if !derrors.IsNotSupported(err) {
		t.Fatalf("MoveMouseToCenterOfWindow() error = %v (code %q), want CodeNotSupported",
			err, derrors.GetCode(err))
	}

	if !strings.Contains(derrors.Message(err), "wayland-wlroots") {
		t.Errorf("error message %q drops what the platform said was missing",
			derrors.Message(err))
	}
}

// TestMoveMouseToCenterOfWindow_ReportsARealFailureAsAccessibilityFailure keeps
// the other half: an error that is not a platform refusal still reads as one
// thing going wrong rather than as a feature this platform never had.
func TestMoveMouseToCenterOfWindow_ReportsARealFailureAsAccessibilityFailure(t *testing.T) {
	sys := &portmocks.MockSystemPort{
		FocusedWindowBoundsFunc: func(context.Context) (image.Rectangle, bool, error) {
			return image.Rectangle{}, false, derrors.New(
				derrors.CodeBridgeFailed,
				"X connection lost",
			)
		},
	}
	service := newTestActionService(&portmocks.MockAccessibilityPort{}, sys)

	err := service.MoveMouseToCenterOfWindow(context.Background(), 0, 0)
	if derrors.GetCode(err) != derrors.CodeAccessibilityFailed {
		t.Fatalf("MoveMouseToCenterOfWindow() code = %q, want %q",
			derrors.GetCode(err), derrors.CodeAccessibilityFailed)
	}
}
