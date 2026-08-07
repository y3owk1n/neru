package modes

import "go.uber.org/zap"

// newHandlerWithState builds a Handler around the given state for tests,
// wiring the outer back-reference and filling in the indicator services the
// way NewHandler does.
func newHandlerWithState(initial handlerState) *Handler {
	// NewHandler guarantees a logger, so the package logs without a nil check
	// anywhere. A test state left one out gets the same guarantee rather than
	// turning a log line into a panic.
	if initial.logger == nil {
		initial.logger = zap.NewNop()
	}

	// NewHandler gives every handler a cell for the focused app, so a test one
	// records what is published to it rather than dropping it.
	if initial.focusedApp == nil {
		initial.focusedApp = &focusedAppCell{}
	}

	handler := new(Handler)
	initial.outer = handler
	handler.handlerState = initial

	fillIndicatorServices(&handler.handlerState, zap.NewNop())

	// Give the handler the same mode map NewHandler builds, so a test that
	// dispatches through a mode reaches the implementation the daemon does.
	// A caller that supplied its own stand-in modes keeps them.
	if handler.modes == nil {
		handler.modes = newModes(&handler.handlerState)
	}

	return handler
}
