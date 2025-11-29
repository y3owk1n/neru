package modes

import (
	"github.com/y3owk1n/neru/internal/core/domain"
)

// ScrollMode implements the Mode interface for scroll-based navigation.
type ScrollMode struct {
	handler *Handler
}

// NewScrollMode creates a new scroll mode implementation.
func NewScrollMode(handler *Handler) *ScrollMode {
	if handler == nil {
		panic("ScrollMode: handler cannot be nil")
	}

	return &ScrollMode{handler: handler}
}

// ModeType returns the domain mode type.
func (m *ScrollMode) ModeType() domain.Mode {
	return domain.ModeScroll
}

// Activate activates scroll mode with optional action parameter.
func (m *ScrollMode) Activate(action *string) {
	if m.handler == nil {
		return
	}
	// Scroll mode ignores the action parameter as it has a single activation flow
	m.handler.StartInteractiveScroll()
}

// HandleKey processes key presses for scroll mode.
func (m *ScrollMode) HandleKey(key string) {
	if m.handler == nil {
		return
	}

	m.handler.handleGenericScrollKey(key)
}

// HandleActionKey processes action keys when in scroll action mode.
// Scroll mode doesn't have action modes like hints and grid.
func (m *ScrollMode) HandleActionKey(key string) {
	// Scroll mode doesn't support action modes
	_ = key // unused parameter
}

// Exit performs scroll mode cleanup.
func (m *ScrollMode) Exit() {
	if m.handler == nil {
		return
	}

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

// ToggleActionMode toggles between overlay and action modes for scroll.
// Scroll mode doesn't have action modes like hints and grid.
func (m *ScrollMode) ToggleActionMode() {
	// Scroll mode doesn't support action mode toggling
}
