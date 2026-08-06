package indicator_test

import (
	"context"
	"errors"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/app/services/indicator"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// TestBase_GetCursorPositionWithoutSystemPort pins the degraded path. The
// service is constructed during startup and can outlive a failed system-port
// init, so a nil port must produce an error rather than a panic.
func TestBase_GetCursorPositionWithoutSystemPort(t *testing.T) {
	t.Parallel()

	base := indicator.NewBase(ports.ModeIndicator, nil, nil)

	_, _, err := base.GetCursorPosition(t.Context())
	if err == nil {
		t.Fatal("GetCursorPosition() error = nil, want an error for a nil system port")
	}

	if !derrors.IsCode(err, derrors.CodeActionFailed) {
		t.Errorf("GetCursorPosition() code = %v, want CodeActionFailed", err)
	}
}

func TestBase_GetCursorPositionReturnsPortCoordinates(t *testing.T) {
	t.Parallel()

	system := &portmocks.MockSystemPort{
		CursorPositionFunc: func(context.Context) (image.Point, error) {
			return image.Point{X: 42, Y: 99}, nil
		},
	}

	base := indicator.NewBase(ports.ModeIndicator, system, nil)

	posX, posY, err := base.GetCursorPosition(t.Context())
	if err != nil {
		t.Fatalf("GetCursorPosition() error = %v, want nil", err)
	}

	if posX != 42 || posY != 99 {
		t.Errorf("GetCursorPosition() = (%d,%d), want (42,99)", posX, posY)
	}
}

// errCursorUnavailable stands in for a backend failure.
var errCursorUnavailable = errors.New("cursor unavailable")

// TestBase_GetCursorPositionWrapsPortFailure pins that a backend error is
// wrapped rather than returned raw, so callers see a domain error code.
func TestBase_GetCursorPositionWrapsPortFailure(t *testing.T) {
	t.Parallel()

	system := &portmocks.MockSystemPort{
		CursorPositionFunc: func(context.Context) (image.Point, error) {
			return image.Point{}, errCursorUnavailable
		},
	}

	base := indicator.NewBase(ports.ModeIndicator, system, nil)

	_, _, err := base.GetCursorPosition(t.Context())
	if err == nil {
		t.Fatal("GetCursorPosition() error = nil, want the wrapped port failure")
	}

	if !errors.Is(err, errCursorUnavailable) {
		t.Errorf("GetCursorPosition() error = %v, want it to wrap %v", err, errCursorUnavailable)
	}
}

// TestBase_VisibilityTargetsItsOwnIndicator pins that each of the three
// visibility calls names the indicator the Base was built for and no other —
// the whole point of one module owning one indicator.
func TestBase_VisibilityTargetsItsOwnIndicator(t *testing.T) {
	t.Parallel()

	for _, kind := range []ports.Indicator{
		ports.ModeIndicator,
		ports.StickyModifiersIndicator,
		ports.VirtualPointerIndicator,
	} {
		t.Run(kind.String(), func(t *testing.T) {
			t.Parallel()

			overlay := &portmocks.MockOverlayPort{}
			base := indicator.NewBase(kind, nil, overlay)

			base.Show()

			if visible, asked := overlay.IndicatorVisible(kind); !asked || !visible {
				t.Errorf("after Show(): visible=%v asked=%v, want both true", visible, asked)
			}

			base.Hide()

			if visible, asked := overlay.IndicatorVisible(kind); !asked || visible {
				t.Errorf(
					"after Hide(): visible=%v asked=%v, want asked and not visible",
					visible,
					asked,
				)
			}

			base.ResizeToActiveScreen()

			if got := overlay.IndicatorResizeCount(kind); got != 1 {
				t.Errorf("resize count = %d, want 1", got)
			}

			for _, other := range []ports.Indicator{
				ports.ModeIndicator,
				ports.StickyModifiersIndicator,
				ports.VirtualPointerIndicator,
			} {
				if other == kind {
					continue
				}

				if _, asked := overlay.IndicatorVisible(other); asked {
					t.Errorf("the %s service touched the %s indicator", kind, other)
				}

				if got := overlay.IndicatorResizeCount(other); got != 0 {
					t.Errorf("the %s service resized the %s indicator", kind, other)
				}
			}
		})
	}
}

// TestBase_WithoutOverlayIsSilent pins the "this indicator was never
// constructed" case — a disabled indicator, or a backend with no surface.
// Mode logic drives an indicator without checking, so the guard has to be
// here.
func TestBase_WithoutOverlayIsSilent(t *testing.T) {
	t.Parallel()

	base := indicator.NewBase(ports.ModeIndicator, nil, nil)

	base.Show()
	base.Hide()
	base.ResizeToActiveScreen()

	if base.Overlay() != nil {
		t.Error("Overlay() must report the missing overlay rather than a stand-in")
	}
}
