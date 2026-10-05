package modes

// baseMode carries what every mode implementation shares: the inner handler
// state a mode runs against. Activate, key handling and exit belong to the
// mode's own type, so that reading that type answers what the mode does.
type baseMode struct {
	handler *handlerState
}

// newBaseMode creates a new base mode with the given handler.
func newBaseMode(handler *handlerState, modeName string) baseMode {
	if handler == nil {
		panic(modeName + ": handler cannot be nil")
	}

	return baseMode{
		handler: handler,
	}
}
