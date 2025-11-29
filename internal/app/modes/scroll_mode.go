package modes

import (
	"github.com/y3owk1n/neru/internal/core/domain"
)

// ScrollMode implements the Mode interface for scroll-based navigation.
type ScrollMode struct {
	baseMode
}

// NewScrollMode creates a new scroll mode implementation.
func NewScrollMode(handler *Handler) *ScrollMode {
	return &ScrollMode{
		baseMode: newBaseMode(handler, domain.ModeScroll, "ScrollMode"),
	}
}

// Activate activates scroll mode with optional action parameter.
func (m *ScrollMode) Activate(_ *string) {
	// Scroll mode ignores the action parameter as it has a single activation flow
	m.handler.StartInteractiveScroll()
}

// HandleKey processes key presses for scroll mode.
func (m *ScrollMode) HandleKey(key string) {
	m.handler.handleGenericScrollKey(key)
}

// Exit performs scroll mode cleanup.
func (m *ScrollMode) Exit() {
	if m.handler.scroll != nil && m.handler.scroll.Context != nil {
		m.handler.scroll.Context.SetIsActive(false)
		m.handler.scroll.Context.SetLastKey("")
	}
	// Reset cursor state when exiting scroll mode to ensure proper cursor restoration
	// in subsequent modes
	if m.handler.cursorState != nil {
		m.handler.cursorState.Reset()
	}
}
