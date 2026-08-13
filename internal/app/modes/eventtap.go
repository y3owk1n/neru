package modes

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/ports"
)

// SetEventTap injects the event tap once infrastructure initialization has
// created it. It mirrors IPCController.SetInfrastructure and must be called
// before any mode is activated.
//
// Every method here tolerates a nil tap: the handler is built in phase 7 and
// the tap only exists in phase 8, and mode setup can run in tests with none.
func (h *Handler) SetEventTap(eventTap ports.EventTapPort) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.eventTap = eventTap
}

// hasEventTap reports whether a tap is wired up, for call sites that need to
// know before doing work rather than relying on the no-op behavior of the
// methods above.
func (h *handlerState) hasEventTap() bool {
	return h.eventTap != nil
}

// eventTapEnabled reports whether keyboard capture is currently live. A handler
// with no tap reads as not capturing, which is what a caller asking whether the
// keyboard has to be given back wants to hear.
func (h *handlerState) eventTapEnabled() bool {
	return h.eventTap != nil && h.eventTap.IsEnabled()
}

// enableEventTap starts keyboard capture.
func (h *handlerState) enableEventTap() {
	if h.eventTap == nil {
		return
	}

	err := h.eventTap.Enable(h.ctx)
	if err != nil {
		h.logger.Error("Failed to enable event tap", zap.Error(err))
	}
}

// disableEventTap stops keyboard capture.
//
// It uses a background context rather than h.ctx because it also runs on the
// cleanup path, after h.ctx has been canceled.
func (h *handlerState) disableEventTap() {
	if h.eventTap == nil {
		return
	}

	err := h.eventTap.Disable(context.Background())
	if err != nil {
		h.logger.Error("Failed to disable event tap", zap.Error(err))
	}
}

// setModifierPassthrough configures whether unbound modifier shortcuts pass
// through to the focused application.
func (h *handlerState) setModifierPassthrough(enabled bool, blacklist []string) {
	if h.eventTap == nil {
		return
	}

	h.eventTap.SetModifierPassthrough(enabled, blacklist)
}

// setInterceptedModifierKeys configures which modifier shortcuts the active
// mode still wants consumed while passthrough is on.
func (h *handlerState) setInterceptedModifierKeys(keys []string) {
	if h.eventTap == nil {
		return
	}

	h.eventTap.SetInterceptedModifierKeys(keys)
}

// setPassthroughCallback registers a function invoked when a modifier shortcut
// passes through. Pass nil to clear.
func (h *handlerState) setPassthroughCallback(callback func()) {
	if h.eventTap == nil {
		return
	}

	h.eventTap.SetPassthroughCallback(callback)
}

// setStickyModifierToggle enables or disables sticky modifier detection.
func (h *handlerState) setStickyModifierToggle(enabled bool) {
	if h.eventTap == nil {
		return
	}

	h.eventTap.SetStickyModifierToggle(enabled)
}

// postModifierEvent simulates a physical modifier press or release.
func (h *handlerState) postModifierEvent(modifier string, isDown bool) {
	if h.eventTap == nil {
		return
	}

	h.eventTap.PostModifierEvent(modifier, isDown)
}

// allowsOverlayKeyboardPassthrough reports whether an indicator overlay may
// give up exclusive keyboard capture so scroll reaches the focused app.
//
// Backends that cannot answer (everything but the Linux Wayland evdev tap) do
// not implement the extension, and the conservative false keeps capture — the
// behavior that works everywhere.
func (h *handlerState) allowsOverlayKeyboardPassthrough() bool {
	reporter, ok := h.eventTap.(ports.OverlayKeyboardPassthroughReporter)

	return ok && reporter.AllowsOverlayKeyboardPassthrough()
}
