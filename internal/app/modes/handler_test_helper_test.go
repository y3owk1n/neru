package modes

import "go.uber.org/zap"

// newHandlerWithState builds a Handler around the given state for tests,
// wiring the outer back-reference and filling in the indicator services the
// way NewHandler does.
func newHandlerWithState(st handlerState) *Handler {
	handler := new(Handler)
	st.outer = handler
	handler.handlerState = st

	fillIndicatorServices(&handler.handlerState, zap.NewNop())

	// Give the handler the same mode map NewHandler builds, so a test that
	// dispatches through a mode reaches the implementation the daemon does.
	// A caller that supplied its own stand-in modes keeps them.
	if handler.modes == nil {
		handler.modes = newModes(&handler.handlerState)
	}

	return handler
}
