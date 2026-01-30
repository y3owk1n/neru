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

			// Start polling for cursor movement to update indicator
			stopCh := make(chan struct{})
			doneCh := make(chan struct{})
			ticker := time.NewTicker(scrollPollInterval)

			handler.scrollStopCh = stopCh
			handler.scrollDoneCh = doneCh
			handler.scrollTicker = ticker

			go func() {
				defer close(doneCh)

				// Create a cancellable context bound to the stop channel
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				// Monitor stopCh to cancel context immediately if the polling operation hangs
				go func() {
					select {
					case <-stopCh:
						cancel()
					case <-ctx.Done():
						// Context canceled by other means (e.g. function exit)
					}
				}()

				for {
					select {
					case <-stopCh:
						return
					case <-ticker.C:
						// Use a timeout for the individual call to prevent hanging
						// 100ms provides a reasonable buffer for the 16ms poll interval
						reqCtx, reqCancel := context.WithTimeout(ctx, scrollPollTimeout)
						cursorX, cursorY, err := handler.scrollService.GetCursorPosition(reqCtx)

						reqCancel()

						if err == nil {
							// Update indicator position
							handler.scrollService.UpdateIndicatorPosition(cursorX, cursorY)
						}
					}
				}
			}()
		},
		ExitFunc: func(handler *Handler) {
			// Signal stop first
			if handler.scrollStopCh != nil {
				close(handler.scrollStopCh)
			}

			// Wait for polling goroutine to finish to avoid race condition where
			// indicator is drawn after cleanup
			if handler.scrollDoneCh != nil {
				<-handler.scrollDoneCh
				handler.scrollDoneCh = nil
			}

			// Clean up resources after loop has exited
			if handler.scrollTicker != nil {
				handler.scrollTicker.Stop()
				handler.scrollTicker = nil
			}

			if handler.scrollStopCh != nil {
				handler.scrollStopCh = nil
			}

			// Explicitly clear and hide the overlay to ensure the scroll indicator is removed
			handler.clearAndHideOverlay()

			if handler.scroll != nil && handler.scroll.Context != nil {
				handler.scroll.Context.SetIsActive(false)
				handler.scroll.Context.SetLastKey("")
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
