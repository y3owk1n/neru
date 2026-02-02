package modes

import (
	"github.com/y3owk1n/neru/internal/core/domain"
)

// ModeBehavior defines the behavior-specific functions for a mode.
type ModeBehavior struct {
	// ActivateFunc handles mode activation (optional, defaults to standard activation)
	ActivateFunc func(handler *Handler, action *string)

	// HandleKeyFunc handles key processing (optional, defaults to standard key handling)
	HandleKeyFunc func(handler *Handler, key string)

	// ExitFunc handles mode cleanup (optional, defaults to standard cleanup)
	ExitFunc func(handler *Handler)
}

// GenericMode provides a generic implementation of the Mode interface
// that can be customized through ModeBehavior.
type GenericMode struct {
	baseMode

	behavior ModeBehavior
}

// NewGenericMode creates a new generic mode with the specified behavior.
func NewGenericMode(
	handler *Handler,
	modeType domain.Mode,
	modeName string,
	behavior ModeBehavior,
) *GenericMode {
	return &GenericMode{
		baseMode: newBaseMode(handler, modeType, modeName),
		behavior: behavior,
	}
}

// Activate activates the mode using the configured behavior or default logic.
func (m *GenericMode) Activate(action *string) {
	if m.behavior.ActivateFunc != nil {
		m.behavior.ActivateFunc(m.handler, action)
	} else {
		// Default activation - try to activate with action
		switch m.modeType {
		case domain.ModeHints:
			m.handler.activateHintModeWithAction(action)
		case domain.ModeGrid:
			m.handler.activateGridModeWithAction(action)
		case domain.ModeQuadGrid:
			m.handler.activateQuadGridModeWithAction(action)
		case domain.ModeScroll:
			m.handler.StartInteractiveScroll()
		case domain.ModeIdle:
			// Idle mode doesn't need activation
		}
	}
}

// HandleKey processes key presses using the configured behavior or default logic.
func (m *GenericMode) HandleKey(key string) {
	if m.behavior.HandleKeyFunc != nil {
		m.behavior.HandleKeyFunc(m.handler, key)
	} else {
		// Default key handling
		switch m.modeType {
		case domain.ModeHints:
			m.handler.handleHintsModeKey(key)
		case domain.ModeGrid:
			m.handler.handleGridModeKey(key)
		case domain.ModeQuadGrid:
			m.handler.handleQuadGridKey(key)
		case domain.ModeScroll:
			m.handler.handleGenericScrollKey(key)
		case domain.ModeIdle:
			// These modes don't handle keys in this context
		}
	}
}

// Exit performs mode cleanup using the configured behavior or default logic.
func (m *GenericMode) Exit() {
	if m.behavior.ExitFunc != nil {
		m.behavior.ExitFunc(m.handler)
	} else {
		// Default cleanup
		switch m.modeType {
		case domain.ModeHints:
			m.handler.cleanupHintsMode()
		case domain.ModeGrid:
			m.handler.cleanupGridMode()
		case domain.ModeQuadGrid:
			m.handler.cleanupQuadGridMode()
		case domain.ModeScroll:
			if m.handler.scroll != nil && m.handler.scroll.Context != nil {
				m.handler.scroll.Context.SetIsActive(false)
				m.handler.scroll.Context.SetLastKey("")
			}

			if m.handler.cursorState != nil {
				m.handler.cursorState.Reset()
			}
		case domain.ModeIdle:
			// Idle mode doesn't need cleanup
		}
	}
}
