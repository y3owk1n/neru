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
// optional extension hints mode opts into (extensions.go).
var (
	_ Mode                   = (*HintsMode)(nil)
	_ cursorFollowSelector   = (*HintsMode)(nil)
	_ exitStepReporter       = (*HintsMode)(nil)
	_ inputEditor            = (*HintsMode)(nil)
	_ hotkeyOverrideReporter = (*HintsMode)(nil)
	_ themeRefresher         = (*HintsMode)(nil)
	_ screenRefresher        = (*HintsMode)(nil)
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

// RefreshForThemeChange draws the labels the session already holds again, so
// they pick up the colors the overlay just re-resolved. Nothing is regenerated:
// the elements have not moved, only the palette changed.
//
// An open search is redrawn with them. The labels and the search box are one
// surface on the backends that paint the box themselves, so redrawing the
// labels alone takes the box off the screen — and a search whose box vanished
// on a theme change looks like a search that ended, while the next key still
// goes to it.
func (m *HintsMode) RefreshForThemeChange() bool {
	handler := m.handler

	if handler.hints == nil || handler.hints.Context == nil {
		return false
	}

	hintCollection := handler.hints.Context.Hints()
	if hintCollection == nil {
		return false
	}

	handler.redrawFrame(
		ports.HintsFrame{
			Screen: handler.screenBounds,
			Hints:  hintCollection.All(),
		},
		"refresh hints after theme change",
	)

	if handler.hints.Context.SearchActive() {
		handler.drawHintSearchInput()
	}

	return true
}

// RefreshForScreenChange regenerates the labels against the display as it now
// is. The arrangement changed under them, so the elements the old labels
// pointed at have moved or gone: the collection is built again from the
// accessibility tree with the session's filters preserved, and nothing left to
// label leaves the mode rather than stranding labels that point nowhere.
//
// Hints switched off in configuration is the one answer that leaves the overlay
// still sized for the display that is gone, so the caller resizes it.
func (m *HintsMode) RefreshForScreenChange(ctx context.Context) bool {
	handler := m.handler

	if handler.config == nil || !handler.config.Hints.Enabled {
		return false
	}

	if handler.refreshHintsForScreenChange(ctx) {
		handler.logger.Debug("Hint overlay resized and regenerated for new screen bounds")
	} else {
		handler.logger.Debug("Hints left the mode during the screen-change refresh")
	}

	return true
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
