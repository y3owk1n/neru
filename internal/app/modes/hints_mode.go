package modes

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// Compile-time interface compliance check.
var _ Mode = (*HintsMode)(nil)

// HintsMode implements the Mode interface for hints-based navigation.
type HintsMode struct {
	baseMode
}

// NewHintsMode creates a new hints mode implementation.
func NewHintsMode(handler *handlerState) *HintsMode {
	return &HintsMode{
		baseMode: newBaseMode(handler, domain.ModeHints, "HintsMode"),
	}
}

// Activate enters hints mode with the flags the activation carries.
func (m *HintsMode) Activate(activation modecmd.Activation) {
	m.handler.activateHintModeWithAction(activation)
}

// HandleKey processes a key press within hints mode.
func (m *HintsMode) HandleKey(key string) {
	m.handler.handleHintsModeKey(key)
}

// RefreshForMonitorMove regenerates the labels against the display the cursor
// landed on. The elements the old labels pointed at are on a screen the user
// has left, so the collection is rebuilt from the accessibility tree with the
// session's filters preserved; nothing to label there exits the mode rather
// than leaving stale labels behind.
func (m *HintsMode) RefreshForMonitorMove(ctx context.Context, targetBounds image.Rectangle) {
	m.handler.refreshHintsForMonitorMove(ctx, targetBounds)
}

// Exit tears hints mode down.
func (m *HintsMode) Exit() {
	m.handler.cleanupHintsMode()
}
