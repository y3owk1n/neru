//go:build !darwin

package hints_test

import (
	"image"
	"testing"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// Off darwin this type paints nothing: hint labels are drawn by the overlay
// manager's own surface (Cairo on Linux, GDI on Windows), and what survives
// here is configuration and a window handle. DrawHints is the one method that
// keeps a signature it cannot honor.
//
// Nothing calls it today — the managers draw through their backends, whose
// no-surface path is pinned separately in
// `internal/adapter/overlay/linux/stub_contract_test.go`. Pinning it anyway is
// what stops the `return nil` becoming true again: hints mode degrades on a
// draw that reports CodeNotSupported and leaves the mode up with no labels,
// while a nil error puts the user in a mode whose targets are invisible and
// unexplained.
//
// The rule is `internal/adapter/platform/AGENTS.md`, "Stubs are loud".
//
// Tagged !darwin, so it runs on both the Linux and the Windows CI leg.
func TestOverlay_DrawHintsReportsNotSupportedOffDarwin(t *testing.T) {
	t.Parallel()

	overlay, err := hints.NewOverlay(config.DefaultConfig().Hints, zap.NewNop())
	if err != nil {
		t.Fatalf("NewOverlay returned error: %v", err)
	}

	drawn := []*hints.Hint{
		hints.NewHint("aa", image.Point{X: 10, Y: 10}, image.Point{X: 20, Y: 10}, ""),
	}

	for _, testCase := range []struct {
		name  string
		hints []*hints.Hint
	}{
		{name: "no hints", hints: nil},
		{name: "real hints", hints: drawn},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			drawErr := overlay.DrawHints(testCase.hints, hints.StyleMode{})
			if drawErr == nil {
				t.Fatal(
					"DrawHints returned nil off darwin; this overlay has no surface, " +
						"so a caller would read success from a draw that painted no labels",
				)
			}

			if !derrors.IsNotSupported(drawErr) {
				t.Errorf("DrawHints returned %v (code %q), want CodeNotSupported",
					drawErr, derrors.GetCode(drawErr))
			}
		})
	}
}

// TestOverlay_DrawHintsRefusalSurvivesAWindow pins that the refusal is about
// this type having no drawing code at all, not about the window handle being
// unset: the manager hands its shared window to NewOverlayWithWindow, and a
// stub that started succeeding once one arrived would put a caller back where
// the test above started.
//
// The handle is a fabricated pointer, which is safe precisely because the stub
// is one — it stores the value and hands it back, and nothing off darwin
// dereferences it.
func TestOverlay_DrawHintsRefusalSurvivesAWindow(t *testing.T) {
	t.Parallel()

	overlay, err := hints.NewOverlayWithWindow(
		config.DefaultConfig().Hints,
		zap.NewNop(),
		unsafe.Pointer(new(int)),
	)
	if err != nil {
		t.Fatalf("NewOverlayWithWindow returned error: %v", err)
	}

	for i := range 3 {
		drawErr := overlay.DrawHints(nil, hints.StyleMode{})
		if !derrors.IsNotSupported(drawErr) {
			t.Fatalf(
				"DrawHints call %d returned %v, want CodeNotSupported every time",
				i+1,
				drawErr,
			)
		}
	}
}
