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

	return handler
}
