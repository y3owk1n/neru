package virtualpointer_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/app/services/virtualpointer"
	"github.com/y3owk1n/neru/internal/ports"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// TestService_UpdateIndicatorPositionForwardsToOverlay pins the draw call. The
// pointer's size and color are the overlay's business, so the position is all
// this service carries.
func TestService_UpdateIndicatorPositionForwardsToOverlay(t *testing.T) {
	t.Parallel()

	overlay := &portmocks.MockOverlayPort{}
	service := virtualpointer.NewService(nil, overlay)

	service.UpdateIndicatorPosition(64, 128)

	gotX, gotY, draws := overlay.LastVirtualPointerPosition()
	if gotX != 64 || gotY != 128 || draws != 1 {
		t.Errorf(
			"DrawVirtualPointer got (%d,%d) after %d draws, want (64,128) after 1",
			gotX, gotY, draws,
		)
	}
}

// TestService_ShowHideDriveTheVirtualPointer pins which indicator this service
// owns. The virtual pointer used to have no service at all, which is why the
// mode handler reached for its render object directly.
func TestService_ShowHideDriveTheVirtualPointer(t *testing.T) {
	t.Parallel()

	overlay := &portmocks.MockOverlayPort{}
	service := virtualpointer.NewService(nil, overlay)

	service.Show()

	if visible, asked := overlay.IndicatorVisible(ports.VirtualPointerIndicator); !asked ||
		!visible {
		t.Errorf("after Show(): visible=%v asked=%v, want both true", visible, asked)
	}

	service.Hide()

	if visible, asked := overlay.IndicatorVisible(ports.VirtualPointerIndicator); !asked ||
		visible {
		t.Errorf("after Hide(): visible=%v asked=%v, want asked and not visible", visible, asked)
	}

	if _, asked := overlay.IndicatorVisible(ports.ModeIndicator); asked {
		t.Error("the virtual pointer service touched the mode indicator")
	}
}

// TestService_WithoutOverlayDrawsNothing pins that a service built for an
// indicator that was never constructed is silent rather than a panic.
func TestService_WithoutOverlayDrawsNothing(t *testing.T) {
	t.Parallel()

	service := virtualpointer.NewService(nil, nil)

	service.Show()
	service.UpdateIndicatorPosition(1, 2)
	service.Hide()

	if service.Overlay() != nil {
		t.Error("Overlay() = non-nil, want nil for a service with no overlay")
	}
}
