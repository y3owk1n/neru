package modes

import (
	"github.com/y3owk1n/neru/internal/core/domain"
)

// ScrollMode implements the Mode interface for scroll-based navigation.
// It uses the generic mode implementation with scroll-specific behavior.
type ScrollMode struct {
	*GenericMode
}

// NewScrollMode creates a new scroll mode implementation.
func NewScrollMode(handler *Handler) *ScrollMode {
	behavior := ModeBehavior{
		ActivateFunc: func(handler *Handler, action *string) {
			// Scroll mode ignores the action parameter as it has a single activation flow
			handler.StartInteractiveScroll()
		},
		ExitFunc: func(handler *Handler) {
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
