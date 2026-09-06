//go:build !darwin

package modeindicator_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/modeindicator"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// Off darwin this type resolves the label and nothing else: the badge is
// painted onto the manager's shared surface (Cairo on Linux, GDI on Windows),
// which is why ResolveLabelText next door is real behavior and
// DrawModeIndicator is not. That split is the whole reason the stub is easy to
// miss — the file it lives in is not otherwise a stub.
//
// Nothing calls it today; the managers draw the badge themselves. Pinning the
// refusal is what keeps the split legible: an indicator whose draw silently
// succeeds is one the user is told is on while the screen shows nothing, and
// on Linux the indicator is the only thing on screen in scroll mode.
//
// The rule is `internal/adapter/platform/AGENTS.md`, "Stubs are loud".
//
// Tagged !darwin, so it runs on both the Linux and the Windows CI leg.
func TestOverlay_DrawModeIndicatorReportsNotSupportedOffDarwin(t *testing.T) {
	t.Parallel()

	overlay, err := modeindicator.NewOverlay(config.DefaultConfig().ModeIndicator, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewOverlay returned error: %v", err)
	}

	// "hints" resolves to a real label and "bogus" resolves to none, so the
	// refusal cannot be mistaken for the empty-label early return that
	// ResolveLabelText owns.
	for _, mode := range []string{"hints", "bogus"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			drawErr := overlay.DrawModeIndicator(mode)
			if drawErr == nil {
				t.Fatalf(
					"DrawModeIndicator(%q) returned nil off darwin; this overlay has no "+
						"surface, so a caller would read success from a badge never painted",
					mode,
				)
			}

			if !derrors.IsNotSupported(drawErr) {
				t.Errorf("DrawModeIndicator(%q) returned %v (code %q), want CodeNotSupported",
					mode, drawErr, derrors.GetCode(drawErr))
			}
		})
	}
}

// TestOverlay_DrawModeIndicatorIsStableAcrossCalls guards against a stub that
// refuses once and then changes its answer: the indicator is redrawn on a
// poll, so a later call reporting success would look like the badge arrived.
func TestOverlay_DrawModeIndicatorIsStableAcrossCalls(t *testing.T) {
	t.Parallel()

	overlay, err := modeindicator.NewOverlay(config.DefaultConfig().ModeIndicator, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewOverlay returned error: %v", err)
	}

	for i := range 3 {
		drawErr := overlay.DrawModeIndicator("hints")
		if !derrors.IsNotSupported(drawErr) {
			t.Fatalf("DrawModeIndicator call %d returned %v, want CodeNotSupported every time",
				i+1, drawErr)
		}
	}
}
