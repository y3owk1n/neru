package modes

import (
	domainHint "github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/ports"
)

// drawHints puts a hint set on screen. It is the hint manager's update
// callback, so it runs on activation, on every keystroke that narrows the
// labels, and on reset.
//
// It hands over a Frame and nothing else: which window calls realize it, and
// in what order, is the overlay's business. The first draw of an activation is
// the transition — the overlay has to be shown and switched — and every draw
// after it is a redraw of a surface already up.
//
// The caller must hold h.mu. SetHints, Reset and HandleInput already do; the
// manager's debounced timer takes it through the mutex it was built with.
func (h *handlerState) drawHints(filteredHints []*domainHint.Interface) {
	if h.overlayPort == nil {
		return
	}

	frame := ports.HintsFrame{
		Screen: h.screenBounds,
		Hints:  filteredHints,
	}

	if h.hintsFrameOnScreen {
		h.redrawFrame(frame, "update hints overlay")

		return
	}

	h.showFrame(frame, "show hints overlay")

	// The overlay was shown and switched whether or not the backend had a
	// surface to draw the labels on, so the next keystroke is a redraw.
	h.hintsFrameOnScreen = true
}
