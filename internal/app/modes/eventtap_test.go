package modes

import (
	"context"
	"testing"

	"go.uber.org/zap"

	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
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

// TestHandler_SetEventTapRoutesToThePort covers the phase-8 injection that
// replaced seven closure fields on the constructor.
func TestHandler_SetEventTapRoutesToThePort(t *testing.T) {
	t.Parallel()

	handler := newEventTapHandler(t)
	tap := &portmocks.MockEventTapPort{}

	handler.SetEventTap(tap)

	if !handler.hasEventTap() {
		t.Fatal("hasEventTap() = false after SetEventTap")
	}

	handler.enableEventTap()

	if !tap.IsEnabled() {
		t.Error("enableEventTap() did not reach the port")
	}

	handler.disableEventTap()

	if tap.IsEnabled() {
		t.Error("disableEventTap() did not reach the port")
	}

	handler.postModifierEvent("cmd", false)

	if got := tap.PostedModifiers(); len(got) != 1 || got[0] != "cmd" {
		t.Errorf("postModifierEvent() posted %v, want [cmd]", got)
	}
}
