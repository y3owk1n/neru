package modeindicator_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/app/services/modeindicator"
	"github.com/y3owk1n/neru/internal/ports"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// TestService_UpdateIndicatorPositionForwardsToOverlay pins the draw call, the
// half of the indicator's life this service adds to the shared one.
func TestService_UpdateIndicatorPositionForwardsToOverlay(t *testing.T) {
	t.Parallel()

	overlay := &portmocks.MockOverlayPort{}
	service := modeindicator.NewService(nil, overlay)

	service.UpdateIndicatorPosition(7, 11)

	gotX, gotY := overlay.LastModeIndicatorPosition()
	if gotX != 7 || gotY != 11 {
		t.Errorf("DrawModeIndicator got (%d,%d), want (7,11)", gotX, gotY)
	}
}

// TestService_ShowHideDriveTheModeIndicator pins which indicator this service
// owns: showing and hiding it must never reach another one.
func TestService_ShowHideDriveTheModeIndicator(t *testing.T) {
	t.Parallel()

	overlay := &portmocks.MockOverlayPort{}
	service := modeindicator.NewService(nil, overlay)

	service.Show()

	if visible, asked := overlay.IndicatorVisible(ports.ModeIndicator); !asked || !visible {
		t.Errorf("after Show(): visible=%v asked=%v, want both true", visible, asked)
	}

	service.Hide()

	if visible, asked := overlay.IndicatorVisible(ports.ModeIndicator); !asked || visible {
		t.Errorf("after Hide(): visible=%v asked=%v, want asked and not visible", visible, asked)
	}
}

// TestService_WithoutOverlayDrawsNothing pins that a service built for an
// indicator that was never constructed is silent rather than a panic: mode
// logic calls it unconditionally.
func TestService_WithoutOverlayDrawsNothing(t *testing.T) {
	t.Parallel()

	service := modeindicator.NewService(nil, nil)

	service.Show()
	service.UpdateIndicatorPosition(1, 2)
	service.Hide()

	if service.Overlay() != nil {
		t.Error("Overlay() = non-nil, want nil for a service with no overlay")
	}
}
