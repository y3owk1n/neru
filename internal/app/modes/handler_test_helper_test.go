package modes

// newHandlerWithState builds a Handler around the given state for tests,
// wiring the outer back-reference the way NewHandler does.
func newHandlerWithState(st handlerState) *Handler {
	handler := new(Handler)
	st.outer = handler
	handler.handlerState = st

	return handler
}
