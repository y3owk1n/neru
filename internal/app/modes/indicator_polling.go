package modes

import (
	"context"
	"image"
	"time"

	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/ui/overlay"
)

const (
	indicatorPollInterval = 16 * time.Millisecond
	indicatorPollTimeout  = 100 * time.Millisecond
)

// startIndicatorPolling starts a goroutine that polls the cursor position and
// updates the mode indicator and sticky modifiers indicator overlays.
// It is shared by all navigation modes (hints, grid, recursive grid, scroll).
func (h *Handler) startIndicatorPolling(mode domain.Mode) {
	// If polling is already active, do not start another goroutine.
	if h.indicatorTicker != nil || h.indicatorStopCh != nil {
		return
	}
	// Only start polling if at least one of mode indicator or sticky modifiers
	// indicator is enabled for this mode.
	if h.config == nil || (!h.modeIndicatorEnabled(mode) && !h.stickyModifiersEnabled()) {
		return
	}
	// Disable exclusive keyboard so scroll events pass through to applications
	// when indicator overlays are shown
	if m := overlay.Get(); m != nil {
		m.SetKeyboardCaptureEnabled(false)
	}
	// Ensure the mode indicator overlay covers the correct screen before
	// the goroutine starts drawing. Scroll and hints modes already call
	// overlayManager.ResizeToActiveScreen() which covers this, but grid
	// and recursive-grid modes manage their own windows and skip that
	// call, so the mode indicator overlay could still be sized for a
	// different monitor.
	if ind := h.overlayManager.ModeIndicatorOverlay(); ind != nil {
		ind.ResizeToActiveScreen()
	}

	if stickyInd := h.overlayManager.StickyModifiersOverlay(); stickyInd != nil {
		stickyInd.ResizeToActiveScreen()
	}

	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	ticker := time.NewTicker(indicatorPollInterval)
	h.indicatorStopCh = stopCh
	h.indicatorDoneCh = doneCh

	h.indicatorTicker = ticker
	go func() {
		defer close(doneCh)
		// Create a cancellable context bound to the stop channel.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// Monitor stopCh to cancel context immediately if the polling operation hangs.
		go func() {
			select {
			case <-stopCh:
				cancel()
			case <-ctx.Done():
			}
		}()

		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				// Re-check stopCh before doing any work to minimize the
				// window where a draw can be dispatched after stop is
				// signaled.
				select {
				case <-stopCh:
					return
				default:
				}
				// Use a timeout for the individual call to prevent hanging.
				reqCtx, reqCancel := context.WithTimeout(ctx, indicatorPollTimeout)
				cursorX, cursorY, err := h.modeIndicatorService.GetCursorPosition(reqCtx)

				reqCancel()

				if err != nil {
					continue
				}

				// Use TryLock to avoid deadlocking with stopIndicatorPolling,
				// which is called while h.mu is held (e.g. exitModeLocked →
				// performCommonCleanup → stopIndicatorPolling blocks on
				// indicatorDoneCh). If the lock is contended, skip this tick.
				if !h.mu.TryLock() {
					continue
				}
				showModeInd := h.shouldShowModeIndicator(h.appState.CurrentMode())
				stickyEnabled := h.stickyModifiersEnabled()
				stickyPoint := h.stickyIndicatorAnchorLocked(image.Pt(cursorX, cursorY))

				cursorPt := image.Pt(cursorX, cursorY)

				if !cursorPt.In(h.screenBounds) && h.system != nil {
					boundsCtx, boundsCancel := context.WithTimeout(ctx, indicatorPollTimeout)
					newBounds, boundsErr := h.system.ScreenBounds(boundsCtx)

					boundsCancel()

					if boundsErr == nil && newBounds != h.screenBounds {
						h.screenBounds = newBounds
						// Must unlock before resizing overlays — the resize
						// dispatches to the main queue and we must not hold
						// h.mu across that call.
						h.mu.Unlock()

						if ind := h.overlayManager.ModeIndicatorOverlay(); ind != nil {
							ind.ResizeToActiveScreen()
						}

						if stickyInd := h.overlayManager.StickyModifiersOverlay(); stickyInd != nil {
							stickyInd.ResizeToActiveScreen()
						}
						// Skip the draw this tick — the async resize hasn't
						// completed yet, so drawing now would target the old
						// window frame. The next tick will draw correctly.
						continue
					}
				}

				screenOrigin := h.screenBounds.Min

				h.mu.Unlock()

				localCursorX := cursorX - screenOrigin.X
				localCursorY := cursorY - screenOrigin.Y
				localStickyX := stickyPoint.X - screenOrigin.X
				localStickyY := stickyPoint.Y - screenOrigin.Y

				// Mode indicator: show and draw when enabled, hide otherwise.
				if showModeInd {
					if ind := h.overlayManager.ModeIndicatorOverlay(); ind != nil {
						ind.Show()
					}

					h.modeIndicatorService.UpdateIndicatorPosition(localCursorX, localCursorY)
				} else if ind := h.overlayManager.ModeIndicatorOverlay(); ind != nil {
					ind.Clear()
					ind.Hide()
				}

				// Sticky modifiers indicator: show and draw when modifiers
				// are active, hide otherwise.
				if stickyEnabled {
					if h.stickyModifiers() != 0 {
						if stickyInd := h.overlayManager.StickyModifiersOverlay(); stickyInd != nil {
							stickyInd.Show()
						}

						h.drawStickyModifiersIndicator(localStickyX, localStickyY)
					} else if stickyInd := h.overlayManager.StickyModifiersOverlay(); stickyInd != nil {
						if h.stickyIndicatorService != nil {
							h.stickyIndicatorService.UpdateIndicatorPosition(
								localStickyX,
								localStickyY,
								"",
							)
						}

						stickyInd.Clear()
						stickyInd.Hide()
					}
				}
			}
		}
	}()
}

// stopIndicatorPolling stops the indicator polling goroutine and cleans up
// both mode indicator and sticky modifiers indicator overlays.
func (h *Handler) stopIndicatorPolling() {
	// Signal stop first.
	if h.indicatorStopCh != nil {
		close(h.indicatorStopCh)
		h.indicatorStopCh = nil
	}
	// Wait for polling goroutine to finish to avoid race conditions where
	// the indicator is drawn after cleanup.
	if h.indicatorDoneCh != nil {
		<-h.indicatorDoneCh
		h.indicatorDoneCh = nil
	}
	// Clear and hide the mode indicator overlay AFTER the goroutine has fully
	// stopped. This ensures any late draw dispatched by the last tick is
	// overridden, preventing the indicator from persisting on screen.
	if ind := h.overlayManager.ModeIndicatorOverlay(); ind != nil {
		ind.Clear()
		ind.Hide()
	}
	// Also clear and hide the sticky modifiers indicator.
	if stickyInd := h.overlayManager.StickyModifiersOverlay(); stickyInd != nil {
		stickyInd.Clear()
		stickyInd.Hide()
	}
	// Clean up resources after loop has exited.
	if h.indicatorTicker != nil {
		h.indicatorTicker.Stop()
		h.indicatorTicker = nil
	}
}

func (h *Handler) modeIndicatorEnabled(mode domain.Mode) bool {
	if h.config == nil {
		return false
	}

	switch mode {
	case domain.ModeIdle:
		return false
	case domain.ModeScroll:
		return h.config.ModeIndicator.Scroll.Enabled
	case domain.ModeHints:
		return h.config.ModeIndicator.Hints.Enabled
	case domain.ModeGrid:
		return h.config.ModeIndicator.Grid.Enabled
	case domain.ModeRecursiveGrid:
		return h.config.ModeIndicator.RecursiveGrid.Enabled
	default:
		return false
	}
}

func (h *Handler) shouldShowModeIndicator(mode domain.Mode) bool {
	return h.modeIndicatorEnabled(mode)
}

func (h *Handler) stickyIndicatorAnchorLocked(cursorPoint image.Point) image.Point {
	switch h.appState.CurrentMode() {
	case domain.ModeGrid:
		if h.grid == nil || h.grid.Context == nil || h.grid.Context.CursorFollowSelection() {
			return cursorPoint
		}

		if selectionPoint, ok := h.grid.Context.SelectionPoint(); ok {
			return selectionPoint
		}
	case domain.ModeRecursiveGrid:
		if h.recursiveGrid == nil || h.recursiveGrid.Context == nil ||
			h.recursiveGrid.Context.CursorFollowSelection() {
			return cursorPoint
		}

		if selectionPoint, ok := h.recursiveGrid.Context.SelectionPoint(); ok {
			return selectionPoint
		}
	case domain.ModeIdle:
	case domain.ModeHints:
	case domain.ModeScroll:
	}

	return cursorPoint
}
