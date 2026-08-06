package modes

import (
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// Compile-time interface compliance check.
var _ Mode = (*ScrollMode)(nil)

// ScrollMode implements the Mode interface for scroll-based navigation.
type ScrollMode struct {
	baseMode
}

// NewScrollMode creates a new scroll mode implementation.
func NewScrollMode(handler *handlerState) *ScrollMode {
	return &ScrollMode{
		baseMode: newBaseMode(handler, domain.ModeScroll, "ScrollMode"),
	}
}

// Activate enters scroll mode.
//
// Scroll mode reads no flags of its own: --toggle is the only one it accepts,
// and the handler answers that before a mode is reached.
func (m *ScrollMode) Activate(_ modecmd.Activation) {
	m.handler.startInteractiveScroll()
	m.handler.startIndicatorPolling(domain.ModeScroll)
}

// HandleKey processes a key press within scroll mode.
func (m *ScrollMode) HandleKey(key string) {
	m.handler.handleGenericScrollKey(key)
}

// Exit tears scroll mode down.
func (m *ScrollMode) Exit() {
	// Common cleanup takes the frame off the screen; stopping the poller
	// first keeps a late tick from putting an indicator back.
	m.handler.stopIndicatorPolling()
	m.handler.stopHeldRepeat()

	if m.handler.scroll != nil && m.handler.scroll.Context != nil {
		m.handler.scroll.Context.Reset()
	}
	// Reset cursor state when exiting scroll mode to ensure proper cursor
	// restoration in subsequent modes.
	if m.handler.cursorState != nil {
		m.handler.cursorState.Reset()
	}
}
