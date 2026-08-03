package modes

import (
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	domainHint "github.com/y3owk1n/neru/internal/domain/hint"
)

// drawHintsLocked renders a hint set onto the overlay. It is the hint manager's
// update callback, so it runs whenever the visible set changes — on activation,
// on every keystroke that narrows the labels, and on reset.
//
// The caller must hold h.mu. The synchronous call sites (SetHints, Reset,
// HandleInput) already hold it; the manager's debounced update timer acquires it
// through the mutex the manager was constructed with.
func (h *Handler) drawHintsLocked(filteredHints []*domainHint.Interface) {
	if h.hints.Overlay == nil {
		return
	}

	// Hints arrive in global screen coordinates; the overlay draws in
	// coordinates local to the screen it covers.
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
		h.currentHintStyleLocked(),
	)
	if drawHintsErr != nil {
		h.logger.Error("Failed to update hints overlay", zap.Error(drawHintsErr))
	}
}
