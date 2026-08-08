//go:build darwin

package darwin_test

import (
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/darwin"
	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// TestManager_DrawHintsWithStyle_UnknownPlacementTravelsAsNotSupported pins the
// code a placement refusal arrives with. The renderer reports CodeNotSupported
// for a placement it cannot draw (#1333), and the mode handler degrades quietly
// on exactly that code — hints mode stays up, with a debug line instead of an
// overlay error. Wrapping it here as an overlay failure would turn a
// degradation into a fault report, which is why the refusal travels unwrapped,
// the same way the overlay adapter already passes one on.
func TestManager_DrawHintsWithStyle_UnknownPlacementTravelsAsNotSupported(t *testing.T) {
	t.Parallel()

	// A placement the vocabulary does not declare, which is the only way to
	// make the renderer refuse. That every declared placement still draws is
	// the renderer's own test to make (render/hints/overlay_darwin_test.go);
	// this one is about the code the refusal arrives with.
	const unknownPlacement = "floating"

	cfg := config.DefaultConfig()
	cfg.Hints.UI.Placement = unknownPlacement

	// Built directly rather than through BuildComponents: the hints overlay
	// shares the manager's window, and this draw never reaches one.
	hintOverlay, err := hints.NewOverlayWithWindow(cfg.Hints, zap.NewNop(), nil)
	if err != nil {
		t.Fatalf("NewOverlayWithWindow() error = %v", err)
	}

	mgr := &darwin.Manager{Base: manager.NewBase(zap.NewNop())}
	mgr.UseHintOverlay(hintOverlay)

	drawn := []*hints.Hint{hints.NewHint("AA", image.Pt(100, 100), image.Pt(20, 20), "")}

	drawErr := mgr.DrawHintsWithStyle(drawn, hints.BuildStyle(cfg.Hints, nil))
	if !derrors.IsNotSupported(drawErr) {
		t.Errorf(
			"DrawHintsWithStyle with an unrecognized placement = %v (code %q), "+
				"want CodeNotSupported: the caller reports it as an overlay failure otherwise",
			drawErr, derrors.GetCode(drawErr),
		)
	}

	// A declared placement still comes back clean through the same wrapper. A
	// pass-through that answered every error alike would pass the assertion
	// above and report a real draw failure as degradation.
	cfg.Hints.UI.Placement = config.HintPlacementDefault

	drawnErr := mgr.DrawHintsWithStyle(drawn, hints.BuildStyle(cfg.Hints, nil))
	if drawnErr != nil {
		t.Errorf("DrawHintsWithStyle(%q) = %v, want nil", config.HintPlacementDefault, drawnErr)
	}
}
