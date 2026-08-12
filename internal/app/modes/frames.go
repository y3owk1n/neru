package modes

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ports"
)

// showFrame puts a Frame on screen: the overlay is sized to the active
// screen, shown, switched to the frame's mode and drawn, in that order and in
// one call, because the order is the adapter's business and not a mode's.
//
// It reports whether the frame reached the screen. Grid mode is the one caller
// that acts on it: a grid that cannot be drawn is a grid nobody can aim with,
// and abandoning the activation is what it has always done there. Recursive
// grid ignores it, also as it always has. A backend with no surface says
// CodeNotSupported and that counts as not reaching the screen; a handler built
// without a port is a test, which has no screen to miss.
//
// The caller must hold h.mu. A draw may block, which is why every caller here
// is either a mode transition or one of the documented locked-context
// exceptions (`internal/app/modes/AGENTS.md`).
func (h *handlerState) showFrame(frame ports.Frame, operation string) bool {
	if h.overlayPort == nil {
		return true
	}

	return h.showFrameResult(frame, operation) == nil
}

// showFrameResult puts a Frame on screen and hands back what went wrong, so a
// mode that must tell the user it cannot run here can tell a backend without
// the surface apart from a draw that failed.
//
// Monitor-select is the only caller that needs the distinction: drawing it is
// an optional capability, and a backend without it owes the user a
// notification rather than a mode that silently refuses to engage. That is
// also why a missing port is CodeNotSupported here rather than the benign
// nothing showFrame makes of it: no port is no surface, and a mode that has to
// report an unsupported surface must not be told the picker is up.
func (h *handlerState) showFrameResult(frame ports.Frame, operation string) error {
	if h.overlayPort == nil {
		return derrors.New(derrors.CodeNotSupported, "no overlay to draw on")
	}

	err := h.overlayPort.ShowFrame(h.ctx, frame)
	h.reportFrameError(err, operation)

	return err
}

// redrawFrame draws a Frame whose overlay is already up, reporting a failure
// under the operation name the caller gives.
//
// Unlike showFrame it answers nothing: a redraw happens over a mode that is
// already running, so there is no activation left to abandon and no caller has
// a different course of action for a failed one. Whether the backend had a
// surface for it is in the log.
func (h *handlerState) redrawFrame(frame ports.Frame, operation string) {
	if h.overlayPort == nil {
		return
	}

	h.reportFrameError(h.overlayPort.RedrawFrame(h.ctx, frame), operation)
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

	h.clearOverlayFrameForRedraw()
}

// clearOverlayFrameForRedraw takes the frame off the screen for a mode that is
// still running and will put it back — a monitor move, which clears before the
// cursor warps so the drawing does not linger on the display being left.
//
// It deliberately leaves hintsFrameOnScreen alone. That flag may only be
// cleared where the hint manager's pending debounced update is invalidated in
// the same locked section (`handler.go`), and this cannot invalidate one: the
// mode is live and its debounce timer may still fire. The redraw on the other
// side clears the flag immediately before SetHints, which is the pattern that
// rule describes.
func (h *handlerState) clearOverlayFrameForRedraw() {
	if h.overlayPort == nil {
		return
	}

	clearErr := h.overlayPort.ClearFrame(context.Background())
	if clearErr != nil {
		h.logger.Error("Failed to clear overlay", zap.Error(clearErr))
	}
}

// updateGridMatches narrows the grid on screen to what the user has typed.
//
// This and the three below are ADR 0003's other half: the updates that fire on
// every keystroke in a grid mode stay plain calls, because building a frame
// and having the adapter diff it is the latency the grid does not have to
// spend. Each is a one-line guard around the port so no caller carries the
// "was there a port" question.
func (h *handlerState) updateGridMatches(prefix string) {
	if h.overlayPort == nil {
		return
	}

	h.overlayPort.UpdateGridMatches(prefix)
}

// setGridHideUnmatched says whether cells that no longer match disappear.
func (h *handlerState) setGridHideUnmatched(hide bool) {
	if h.overlayPort == nil {
		return
	}

	h.overlayPort.SetGridHideUnmatched(hide)
}

// showGridSubgrid opens the finer grid drawn inside one cell, with the pointer
// stand-in the selection that opened it asks for.
//
// The pointer travels with the open rather than in a call of its own (#1492).
// The keystroke that picks a cell is the keystroke that moves the selection, and
// on a backend that paints both into one surface two calls are two repaints of
// it — arrow keys inside a subgrid included, once per repeat of a held key.
func (h *handlerState) showGridSubgrid(cell *domainGrid.Cell, pointer ports.GridPointer) {
	if h.overlayPort == nil {
		return
	}

	h.overlayPort.ShowGridSubgrid(cell, pointer)
}

// updateGridPointer moves the pointer stand-in on a grid mode's surface, or
// takes it off.
func (h *handlerState) updateGridPointer(mode domain.Mode, pointer ports.GridPointer) {
	if h.overlayPort == nil {
		return
	}

	h.overlayPort.UpdateGridPointer(mode, pointer)
}

// reportFrameError logs a frame failure. A backend without a surface for the
// frame says CodeNotSupported, which is degradation rather than failure — and
// so does a backend that has the surface but cannot place what the frame
// carries, such as a hint placement it has no branch for (#1333).
func (h *handlerState) reportFrameError(err error, operation string) {
	if err == nil {
		return
	}

	if derrors.IsNotSupported(err) {
		// Either the backend has no surface for this frame or it could not
		// place the content on one; both mean nothing is on screen, and the
		// overlay says which it was.
		h.logger.Debug("Overlay frame not drawn",
			zap.String("operation", operation))

		return
	}

	h.logger.Error("Overlay frame failed",
		zap.String("operation", operation),
		zap.Error(err))
}
