package modes

import (
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/derrors"
	domainHint "github.com/y3owk1n/neru/internal/domain/hint"
)

// drawHints renders a hint set onto the overlay. It is the manager's
// update callback, so it runs on activation, on every keystroke that narrows
// the labels, and on reset.
//
// The caller must hold h.mu. SetHints, Reset and HandleInput already do; the
// manager's debounced timer takes it through the mutex it was built with.
func (h *handlerState) drawHints(filteredHints []*domainHint.Interface) {
	if h.overlayManager == nil {
		return
	}

	// Hints are in global screen coordinates; the overlay draws in local ones.
	screenBounds := h.screenBounds

	overlayHints := make([]*hints.Hint, len(filteredHints))
	for index, hint := range filteredHints {
		localPos := image.Point{
			X: hint.Position().X - screenBounds.Min.X,
			Y: hint.Position().Y - screenBounds.Min.Y,
		}
		overlayHints[index] = hints.NewHint(
			hint.Label(),
			localPos,
			hint.Element().Bounds().Size(),
			hint.MatchedPrefix(),
		)
	}

	drawHintsErr := h.overlayManager.DrawHintsWithStyle(
		overlayHints,
		h.currentHintStyle(),
	)
	if drawHintsErr != nil {
		// A backend without a hint surface (headless) reports CodeNotSupported;
		// that is degradation, not failure.
		if derrors.IsNotSupported(drawHintsErr) {
			h.logger.Debug("Hint overlay not supported on this backend")

			return
		}

		h.logger.Error("Failed to update hints overlay", zap.Error(drawHintsErr))
	}
}
