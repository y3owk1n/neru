package modes

// HandleKeyPress dispatches key events by current mode.
func (h *Handler) HandleKeyPress(key string) {
	// Process any pending hints refresh from timer callback (dispatched to main thread)
	select {
	case <-h.refreshHintsCh:
		h.activateHintModeInternal(false, nil)
	default:
		// No pending refresh
	}

	if key == KeyEscape || key == KeyEscape2 {
		h.handleEscapeKey()

		return
	}

	h.handleModeSpecificKey(key)
}

// handleEscapeKey handles the escape key to exit the current mode.
func (h *Handler) handleEscapeKey() {
	_, exists := h.modes[h.appState.CurrentMode()]
	if !exists {
		return
	}

	h.ExitMode()
	h.SetModeIdle()
}

// handleModeSpecificKey handles mode-specific key processing.
func (h *Handler) handleModeSpecificKey(key string) {
	mode, exists := h.modes[h.appState.CurrentMode()]
	if !exists {
		return
	}

	mode.HandleKey(key)
}
