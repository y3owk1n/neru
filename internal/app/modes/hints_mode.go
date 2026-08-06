package modes

import (
	"context"
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// Compile-time interface compliance checks: the core interface, then every
// optional extension hints mode opts into (extensions.go).
var (
	_ Mode                   = (*HintsMode)(nil)
	_ cursorFollowSelector   = (*HintsMode)(nil)
	_ exitStepReporter       = (*HintsMode)(nil)
	_ inputEditor            = (*HintsMode)(nil)
	_ hotkeyOverrideReporter = (*HintsMode)(nil)
)

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

// CursorFollowSelection reports whether the hint session moves the real cursor
// onto the hints it selects.
func (m *HintsMode) CursorFollowSelection() (bool, bool) {
	modeContext, ok := m.cursorFollowContext()
	if !ok {
		return false, false
	}

	return modeContext.CursorFollowSelection(), true
}

// ApplyCursorFollowSelection sets or toggles the preference.
//
// Unlike the grids, hints owes the change nothing after it: it draws no virtual
// pointer and holds no selection point to jump the cursor to, so the preference
// only affects the selections made after it.
func (m *HintsMode) ApplyCursorFollowSelection(desired *bool) (bool, bool) {
	modeContext, ok := m.cursorFollowContext()
	if !ok {
		return false, false
	}

	return applyCursorFollow(modeContext, desired), true
}

// ExitSteps reports the --on-exit steps this hint session was activated with.
func (m *HintsMode) ExitSteps() []string {
	if m.handler.hints == nil || m.handler.hints.Context == nil {
		return nil
	}

	return m.handler.hints.Context.OnExit()
}

// ResetInput does nothing, and that is hints mode's answer rather than an
// absence: the labels are generated from the accessibility tree once per
// activation, so there is no input state to clear that would not amount to
// re-entering the mode.
func (m *HintsMode) ResetInput() {}

// Backspace takes back the last typed character of the hint label being
// entered, and forgets where a cycle_hint walk had got to.
func (m *HintsMode) Backspace() {
	handler := m.handler

	if handler.hints != nil && handler.hints.Context != nil &&
		handler.hints.Context.Manager() != nil {
		backspaceErr := handler.hints.Context.Manager().HandleBackspace()
		if backspaceErr != nil {
			handler.logger.Error("Hint backspace failed", zap.Error(backspaceErr))
		}
	}

	handler.cycleHintIndex = -1
}

// HasAppHotkeyOverrides reports whether [hints.apps] binds any per-app hotkey.
func (m *HintsMode) HasAppHotkeyOverrides() bool {
	if m.handler.config == nil {
		return false
	}

	return m.handler.config.Hints.HasAppHotkeyOverrides()
}

// cursorFollowContext is the hint session's preference carrier, or false when
// there is no session.
func (m *HintsMode) cursorFollowContext() (cursorFollowContext, bool) {
	if m.handler.hints == nil || m.handler.hints.Context == nil {
		return nil, false
	}

	return m.handler.hints.Context, true
}
