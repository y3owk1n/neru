package indicator_test

import (
	"context"
	"errors"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/app/services/indicator"
	"github.com/y3owk1n/neru/internal/ports"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// errCursorUnavailable stands in for a backend failure.
var errCursorUnavailable = errors.New("cursor unavailable")

// TestBase_GetCursorPositionReturnsPortCoordinates pins the cursor read every
// position update begins with. A nil port (the service can outlive a failed
// system-port init) and a failing port both answer an error, never a panic.
func TestBase_GetCursorPositionReturnsPortCoordinates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		system  ports.SystemPort
		want    image.Point
		wantErr bool
	}{
		{
			name: "port coordinates",
			system: &portmocks.MockSystemPort{
				CursorPositionFunc: func(context.Context) (image.Point, error) {
					return image.Point{X: 42, Y: 99}, nil
				},
			},
			want: image.Point{X: 42, Y: 99},
		},
		{name: "no system port", wantErr: true},
		{
			name: "port failure",
			system: &portmocks.MockSystemPort{
				CursorPositionFunc: func(context.Context) (image.Point, error) {
					return image.Point{}, errCursorUnavailable
				},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			base := indicator.NewBase(ports.ModeIndicator, test.system, nil)

			posX, posY, err := base.GetCursorPosition(t.Context())
			if (err != nil) != test.wantErr {
				t.Fatalf("GetCursorPosition() error = %v, wantErr %v", err, test.wantErr)
			}

			if got := (image.Point{X: posX, Y: posY}); got != test.want {
				t.Errorf("GetCursorPosition() = %v, want %v", got, test.want)
			}
		})
	}
}

// TestBase_VisibilityTargetsItsOwnIndicator pins that each of the three
// visibility calls names the indicator the Base was built for and no other.
// That is the whole point of one module owning one indicator.
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
