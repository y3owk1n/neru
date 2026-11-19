package modes

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

// newEventTapHandler builds the minimum Handler needed to drive the event-tap
// helpers: a context, a logger, and nothing else.
func newEventTapHandler(t *testing.T) *Handler {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	return newHandlerWithState(handlerState{ctx: ctx, logger: zap.NewNop()})
}

// TestHandler_EventTapHelpersTolerateNilTap pins the nil-safety the whole
// design depends on. The handler is built in initialization phase 7 and the
// event tap only arrives in phase 8, so every helper must be callable before
// SetEventTap and in tests that never wire one.
func TestHandler_EventTapHelpersTolerateNilTap(t *testing.T) {
	t.Parallel()

	handler := newEventTapHandler(t)

	if handler.hasEventTap() {
		t.Error("hasEventTap() = true with no tap injected")
	}

	// None of these may panic.
	handler.enableEventTap()
	handler.disableEventTap()
	handler.setModifierPassthrough(true, []string{"Cmd+Q"})
	handler.setInterceptedModifierKeys([]string{"Cmd+W"})
	handler.setPassthroughCallback(func() {})
	handler.setStickyModifierToggle(true)
	handler.postModifierEvent("cmd", true)
}
