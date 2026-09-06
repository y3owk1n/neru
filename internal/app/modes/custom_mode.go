package modes

import (
	"context"
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/ports"
)

// Compile-time interface compliance checks: the core interface, then every
// optional extension a declared mode opts into (extensions.go).
var (
	_ Mode                   = (*CustomMode)(nil)
	_ hotkeyOverrideReporter = (*CustomMode)(nil)
)

// CustomMode implements the Mode interface for every mode the user declared
// under [modes.<name>]. One implementation serves them all: a declared mode
// has no logic of its own, so what tells one from another is the name on the
// handler, which picks the keymap, the indicator label and the frame.
//
// It is scroll mode without the scrolling. It draws nothing, captures the
// keyboard, answers every key from the settled keymap, and shows the mode
// indicator with the label the declaration gave it.
type CustomMode struct {
	baseMode
}

// NewCustomMode creates the mode implementation every declared mode runs on.
func NewCustomMode(handler *handlerState) *CustomMode {
	return &CustomMode{
		baseMode: newBaseMode(handler, domain.ModeCustom, "CustomMode"),
	}
}

// Activate enters the declared mode the activation names.
//
// --toggle is the only flag a declared mode accepts, and the handler answers
// that before a mode is reached. The name is checked against the
// configuration here as well as at the IPC boundary, because an internal
// caller can build an activation the grammar never saw.
func (m *CustomMode) Activate(activation modecmd.Activation) {
	m.handler.startCustomMode(activation.Name)
}

// HandleKey processes a key press within a declared mode. Nothing is done
// here: a declared mode's keys are fully driven by its hotkey table.
func (m *CustomMode) HandleKey(_ string) {}

// RefreshForMonitorMove switches the overlay back to the declared mode on the
// display the cursor landed on, for the reason scroll does: the indicators
// naming the mode are painted on the shared surface on Linux.
func (m *CustomMode) RefreshForMonitorMove(_ context.Context, _ image.Rectangle) {
	m.handler.showFrame(m.handler.customFrame(), "refresh custom mode after monitor move")
}

// Exit tears the declared mode down. The name is cleared when the app state
// leaves ModeCustom, so a late reader still sees which declaration the
// session was in until then.
func (m *CustomMode) Exit() {
	// Common cleanup takes the frame off the screen; stopping the poller
	// first keeps a late tick from putting an indicator back.
	m.handler.stopIndicatorPolling()
	m.handler.stopHeldRepeat()

	if m.handler.cursorState != nil {
		m.handler.cursorState.Reset()
	}
}

// HasAppHotkeyOverrides reports whether the declared mode's [[app_configs]]
// binds any per-app hotkey. It is what lets the keymap settle without asking
// which app is focused when no declaration cares.
func (m *CustomMode) HasAppHotkeyOverrides() bool {
	if m.handler.config == nil {
		return false
	}

	return m.handler.config.Modes[m.handler.appState.CustomModeName()].HasAppHotkeyOverrides()
}

// startCustomMode enters the declared mode called name.
//
// It is scroll's activation with the name set before the mode is entered,
// because entering the mode settles the keymap, and the keymap of a declared
// mode is looked up by that name.
//
// Caller must hold h.mu.
func (h *handlerState) startCustomMode(name string) {
	if h.config == nil {
		return
	}

	if _, declared := h.config.Modes[name]; !declared {
		h.logger.Warn("Custom mode activation refused: mode is not declared",
			zap.String("mode", name))

		return
	}

	h.prepareForModeActivation()
	h.cursorState.SkipNextRestore()

	if h.appState.CurrentMode() != domain.ModeIdle {
		h.cleanupForKeymapModeTransition()

		h.logger.Debug("Transitioned to custom mode",
			zap.String("from", h.CurrModeString()))
	}

	h.appState.SetCustomModeName(name)

	h.enterMode(domain.ModeCustom)

	// Entering a declared mode is a transition like entering scroll: the
	// frame takes the previous mode's drawing off the shared surface and
	// tells the overlay which mode the indicators are naming.
	h.showFrame(h.customFrame(), "show custom mode overlay")
	h.startIndicatorPolling(domain.ModeCustom)

	h.logger.Info("Custom mode activated", zap.String("mode", name))
}

// customFrame is the frame of the declared mode the handler is in.
//
// Caller must hold h.mu.
func (h *handlerState) customFrame() ports.CustomFrame {
	return ports.CustomFrame{Name: h.appState.CustomModeName()}
}

// DeclaresMode reports whether the configuration declares a mode called name,
// for the caller that has to refuse "mode <name>" before it reaches an
// activation that cannot answer.
func (h *Handler) DeclaresMode(name string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.config == nil {
		return false
	}

	_, declared := h.config.Modes[name]

	return declared
}
