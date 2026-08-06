package modes

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/derrors"
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
		h.reportFrameError(
			h.overlayPort.RedrawFrame(h.ctx, frame),
			"update hints overlay",
		)

		return
	}

	h.reportFrameError(h.overlayPort.ShowFrame(h.ctx, frame), "show hints overlay")

	// The overlay was shown and switched whether or not the backend had a
	// surface to draw the labels on, so the next keystroke is a redraw.
	h.hintsFrameOnScreen = true
}

// redrawFrame draws a Frame whose overlay is already up, reporting a failure
// under the operation name the caller gives. It returns whether the overlay
// was asked at all — only a handler built without a port, which is a test,
// is not.
func (h *handlerState) redrawFrame(frame ports.Frame, operation string) bool {
	if h.overlayPort == nil {
		return false
	}

	h.reportFrameError(h.overlayPort.RedrawFrame(h.ctx, frame), operation)

	return true
}

// clearOverlayFrame takes whatever is on screen off it and returns the overlay
// to idle. It also ends the hints frame's life, so the next activation shows
// and switches rather than redrawing a surface that is no longer up.
//
// It uses a background context rather than h.ctx because it also runs on the
// cleanup path, after h.ctx has been canceled — and taking the frame off the
// screen is precisely what must still happen there. Showing a frame is the
// opposite: a canceled context means the activation is over and nothing should
// be drawn, so those paths keep h.ctx.
func (h *handlerState) clearOverlayFrame() {
	h.hintsFrameOnScreen = false

	if h.overlayPort == nil {
		return
	}

	clearErr := h.overlayPort.ClearFrame(context.Background())
	if clearErr != nil {
		h.logger.Error("Failed to clear overlay", zap.Error(clearErr))
	}
}

// reportFrameError logs a frame failure. A backend without a surface for the
// frame says CodeNotSupported, which is degradation rather than failure.
func (h *handlerState) reportFrameError(err error, operation string) {
	if err == nil {
		return
	}

	if derrors.IsNotSupported(err) {
		h.logger.Debug("Overlay frame not supported on this backend",
			zap.String("operation", operation))

		return
	}

	h.logger.Error("Overlay frame failed",
		zap.String("operation", operation),
		zap.Error(err))
}
