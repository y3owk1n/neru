//go:build !darwin

package grid_test

import (
	"image"
	"testing"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
)

// Off darwin this type paints nothing: the grid is drawn by the overlay
// manager's own surface (Cairo on Linux, GDI on Windows), and what survives
// here is configuration and a window handle. DrawGrid is the one method that
// keeps a signature it cannot honor, and it is reachable only through the
// component the manager builds.
//
// Nothing calls it today — the managers draw through their backends, whose
// no-surface path is pinned separately in
// `internal/adapter/overlay/linux/stub_contract_test.go`. That is exactly why
// this is pinned rather than trusted: a `return nil` inside an unused method
// is invisible until the day a backend reaches for the component it was handed
// and reads success from a draw that painted nothing, leaving the mode handler
// with keyboard capture armed over an empty screen.
//
// The rule is `internal/adapter/platform/AGENTS.md`, "Stubs are loud".
//
// Tagged !darwin, so it runs on both the Linux and the Windows CI leg.
func TestOverlay_DrawGridReportsNotSupportedOffDarwin(t *testing.T) {
	t.Parallel()

	overlay, err := grid.NewOverlay(config.DefaultConfig().Grid, zap.NewNop())
	if err != nil {
		t.Fatalf("NewOverlay returned error: %v", err)
	}

	drawn := domainGrid.NewGrid("abc", image.Rect(0, 0, 800, 600), zap.NewNop())

	for _, testCase := range []struct {
		name string
		grid *domainGrid.Grid
	}{
		{name: "no grid", grid: nil},
		{name: "real grid", grid: drawn},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			drawErr := overlay.DrawGrid(testCase.grid)
			if drawErr == nil {
				t.Fatal(
					"DrawGrid returned nil off darwin; this overlay has no surface, " +
						"so a caller would read success from a draw that painted nothing",
				)
			}

			if !derrors.IsNotSupported(drawErr) {
				t.Errorf("DrawGrid returned %v (code %q), want CodeNotSupported",
					drawErr, derrors.GetCode(drawErr))
			}
		})
	}
}

// TestOverlay_DrawGridRefusalSurvivesAWindow pins that the refusal is about
// this type having no drawing code at all, not about the window handle being
// unset: the manager hands its shared window to NewOverlayWithWindow, and a
// stub that started succeeding once one arrived would put a caller back where
// the test above started.
//
// The handle is a fabricated pointer, which is safe precisely because the stub
// is one — it stores the value and hands it back from Window(), and nothing
// off darwin dereferences it.
func TestOverlay_DrawGridRefusalSurvivesAWindow(t *testing.T) {
	t.Parallel()

	overlay := grid.NewOverlayWithWindow(
		config.DefaultConfig().Grid,
		zap.NewNop(),
		unsafe.Pointer(new(int)),
	)

	for i := range 3 {
		err := overlay.DrawGrid(nil)
		if !derrors.IsNotSupported(err) {
			t.Fatalf("DrawGrid call %d returned %v, want CodeNotSupported every time", i+1, err)
		}
	}
}
