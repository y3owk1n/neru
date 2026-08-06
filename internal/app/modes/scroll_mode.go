package modes

import (
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// ScrollMode implements the Mode interface for scroll-based navigation.
// It uses the generic mode implementation with scroll-specific behavior.
type ScrollMode struct {
	*GenericMode
}

// NewScrollMode creates a new scroll mode implementation.
func NewScrollMode(handler *handlerState) *ScrollMode {
	behavior := ModeBehavior{
		ActivateFunc: func(handler *handlerState, _ modecmd.Activation) {
			// Scroll mode reads no flags of its own: --toggle is the only one it
			// accepts, and the handler answers that before a mode is reached.
			handler.startInteractiveScroll()
			handler.startIndicatorPolling(domain.ModeScroll)
		},
		ExitFunc: func(handler *handlerState) {
			// Common cleanup takes the frame off the screen; stopping the
			// poller first keeps a late tick from putting an indicator back.
			handler.stopIndicatorPolling()
			handler.stopHeldRepeat()

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
