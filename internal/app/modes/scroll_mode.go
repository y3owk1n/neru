package modes

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// Compile-time interface compliance checks: the core interface, then every
// optional extension scroll mode opts into (extensions.go).
var (
	_ Mode                   = (*ScrollMode)(nil)
	_ hotkeyOverrideReporter = (*ScrollMode)(nil)
)

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

// RefreshForMonitorMove switches the overlay back to scroll on the display the
// cursor landed on. Scroll draws nothing of its own, but on Linux the
// indicators that name the mode are painted on the shared surface, so the
// surface still has to come back up.
func (m *ScrollMode) RefreshForMonitorMove(_ context.Context, _ image.Rectangle) {
	m.handler.refreshScrollForMonitorMove()
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

// HasAppHotkeyOverrides reports whether [scroll.apps] binds any per-app hotkey.
func (m *ScrollMode) HasAppHotkeyOverrides() bool {
	if m.handler.config == nil {
		return false
	}

	return m.handler.config.Scroll.HasAppHotkeyOverrides()
}
