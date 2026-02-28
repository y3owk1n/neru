package modes

import (
	"context"
	"time"

	"github.com/y3owk1n/neru/internal/core/domain"
)

// ScrollMode implements the Mode interface for scroll-based navigation.
// It uses the generic mode implementation with scroll-specific behavior.
type ScrollMode struct {
	*GenericMode
}

const (
	scrollPollInterval = 16 * time.Millisecond
	scrollPollTimeout  = 100 * time.Millisecond
)

// NewScrollMode creates a new scroll mode implementation.
func NewScrollMode(handler *Handler) *ScrollMode {
	behavior := ModeBehavior{
		ActivateFunc: func(handler *Handler, action *string) {
			// Scroll mode ignores the action parameter as it has a single activation flow
			handler.StartInteractiveScroll()
			handler.startModeIndicatorPolling(domain.ModeScroll)
		},
		ExitFunc: func(handler *Handler) {
			handler.stopModeIndicatorPolling()

			handler.clearAndHideOverlay()

			if handler.scroll != nil && handler.scroll.Context != nil {
				handler.scroll.Context.Reset()
			}
			// Reset cursor state when exiting scroll mode to ensure proper cursor restoration
			// in subsequent modes
			if handler.cursorState != nil {
				handler.cursorState.Reset()
			}
		},
	}

	return &ScrollMode{
		GenericMode: NewGenericMode(handler, domain.ModeScroll, "ScrollMode", behavior),
	}
}

func (h *Handler) startModeIndicatorPolling(mode domain.Mode) {
	// If polling is already active, do not start another goroutine.
	if h.scrollTicker != nil || h.scrollStopCh != nil {
		return
	}

	// Only start polling if the current mode's indicator is enabled.
	if h.config == nil || !h.modeIndicatorEnabled(mode) {
		return
	}

	// Ensure the mode indicator overlay covers the correct screen before
	// the goroutine starts drawing. Scroll and hints modes already call
	// overlayManager.ResizeToActiveScreen() which covers this, but grid
	// and recursive-grid modes manage their own windows and skip that
	// call, so the mode indicator overlay could still be sized for a
	// different monitor.
	if ind := h.overlayManager.ModeIndicatorOverlay(); ind != nil {
		ind.ResizeToActiveScreen()
		ind.Show()
	}

	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	ticker := time.NewTicker(scrollPollInterval)

	h.scrollStopCh = stopCh
	h.scrollDoneCh = doneCh
	h.scrollTicker = ticker

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
				reqCtx, reqCancel := context.WithTimeout(ctx, scrollPollTimeout)
				cursorX, cursorY, err := h.modeIndicatorService.GetCursorPosition(reqCtx)

				reqCancel()

				if err != nil {
					continue
				}

				if !h.shouldShowModeIndicator(h.appState.CurrentMode()) {
					continue
				}

				h.modeIndicatorService.UpdateIndicatorPosition(cursorX, cursorY)
			}
		}
	}()
}

func (h *Handler) stopModeIndicatorPolling() {
	// Signal stop first.
	if h.scrollStopCh != nil {
		close(h.scrollStopCh)
		h.scrollStopCh = nil
	}

	// Wait for polling goroutine to finish to avoid race conditions where
	// the indicator is drawn after cleanup.
	if h.scrollDoneCh != nil {
		<-h.scrollDoneCh
		h.scrollDoneCh = nil
	}

	// Clear and hide the mode indicator overlay AFTER the goroutine has fully
	// stopped. This ensures any late draw dispatched by the last tick is
	// overridden, preventing the indicator from persisting on screen.
	if ind := h.overlayManager.ModeIndicatorOverlay(); ind != nil {
		ind.Clear()
		ind.Hide()
	}

	// Clean up resources after loop has exited.
	if h.scrollTicker != nil {
		h.scrollTicker.Stop()
		h.scrollTicker = nil
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
		return h.config.ModeIndicator.ScrollEnabled
	case domain.ModeHints:
		return h.config.ModeIndicator.HintsEnabled
	case domain.ModeGrid:
		return h.config.ModeIndicator.GridEnabled
	case domain.ModeRecursiveGrid:
		return h.config.ModeIndicator.RecursiveGridEnabled
	default:
		return false
	}
}

func (h *Handler) shouldShowModeIndicator(mode domain.Mode) bool {
	return h.modeIndicatorEnabled(mode)
}
