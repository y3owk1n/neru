package modes

import (
	"context"
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/domain"
)

// This file holds the optional extensions to Mode: the behavior only some
// modes have, expressed as a narrow unexported interface per axis rather than
// as a switch over domain.Mode with empty arms.
//
// The rules — the assertion each implementer owes, absence semantics, and the
// matrix test that replaces what the exhaustive linter gives a mode switch —
// are in the modes area guide (AGENTS.md). Read it before adding an axis.

// extensionName names one optional-extension axis. There is exactly one name
// per axis and it is used twice: in the matrix that states which modes carry
// the axis (extensions_test.go), and in the debug line an effect axis leaves
// behind when a mode declines it. Naming it once is what keeps the log and the
// matrix talking about the same thing.
type extensionName string

const (
	extensionSelectionTracking extensionName = "selection tracking"
	extensionCellNavigation    extensionName = "cell navigation"
	extensionCursorFollow      extensionName = "cursor follow selection"
	extensionExitSteps         extensionName = "exit step reporting"
	extensionInputEditing      extensionName = "input editing"
	extensionHotkeyOverrides   extensionName = "hotkey override reporting"
	extensionThemeRefresh      extensionName = "theme change refresh"
	extensionScreenRefresh     extensionName = "screen change refresh"
)

// selectionTracker is an optional Mode extension: a mode that remembers where
// its selection sits, separately from where the real cursor is.
//
// Grid and recursive grid carry a selection point; hints, scroll and monitor
// select have nothing of the kind, and a mode that does not implement this is
// silently absent from every call site — the same nothing the empty switch
// arms used to say.
type selectionTracker interface {
	// SelectionPoint reports the mode's current selection in global screen
	// coordinates, and false when the mode has no session or nothing is
	// selected in it.
	SelectionPoint() (image.Point, bool)

	// ClearSelectionPoint forgets the selection and brings the mode's virtual
	// pointer back in step with it. It reports false when there is no session
	// to clear.
	ClearSelectionPoint() bool

	// SelectionAnchor reports the point an indicator should be anchored to
	// instead of the cursor: the selection, whenever the real cursor is not
	// already following it. False means "anchor to the cursor".
	//
	// It must stay a pure read. It runs on the indicator poll tick, where the
	// planIndicatorTick/drawIndicators split exists precisely to keep draws
	// off h.mu, so an implementation that reached the overlay would put one
	// back on it.
	SelectionAnchor() (image.Point, bool)
}

// cellNavigator is an optional Mode extension: a mode whose selection sits in
// a cell of a layout it can slide through without changing the active layer.
type cellNavigator interface {
	// MoveCell slides the selection count cells in dir. A move that would
	// leave the screen is the mode's own business to refuse.
	MoveCell(dir domain.Direction, count int)
}

// cursorFollowSelector is an optional Mode extension: a mode whose session
// carries the cursor-follow-selection preference — whether the real cursor is
// dragged along to whatever the mode selects.
//
// Hints, grid and recursive grid each keep it on their own session context.
// Scroll and the monitor picker select nothing of the kind, so there is nothing
// for a cursor to follow, and both results below are the refusal a caller
// already handles.
type cursorFollowSelector interface {
	// CursorFollowSelection reports whether the session follows the selection
	// with the real cursor, and false when the mode has no session to answer
	// for — the same condition under which changing it is refused.
	CursorFollowSelection() (bool, bool)

	// ApplyCursorFollowSelection sets the preference to *desired, or toggles it
	// when desired is nil, and reports the value it settled on.
	//
	// Setting it to the value it already holds still runs whatever the mode
	// owes the change, which is what makes the setter idempotent in the way a
	// caller needs: "on" always ends with the cursor on the selection.
	ApplyCursorFollowSelection(desired *bool) (bool, bool)
}

// exitStepReporter is an optional Mode extension: a mode that can be activated
// with --on-exit steps, the sequence run once its pending action was fulfilled.
//
// Only the modes that carry a pending action have any, so scroll and monitor
// select report nothing and stay silent about it.
type exitStepReporter interface {
	// ExitSteps reports the --on-exit steps the mode's session was activated
	// with, or nil when none were given or there is no session.
	ExitSteps() []string
}

// inputEditor is an optional Mode extension: a mode with accumulated input the
// user can edit in place — clearing it back to the start, or taking back the
// last keystroke — without leaving the mode.
//
// This is an effect rather than a getter, so a mode that does not carry it says
// so in the debug log (activeModeEffect) instead of silently doing nothing.
type inputEditor interface {
	// ResetInput clears the mode's input back to how its session started,
	// leaving the session itself open.
	ResetInput()

	// Backspace takes back the most recent unit of input: a typed character, or
	// for recursive grid a level of zoom.
	Backspace()
}

// hotkeyOverrideReporter is an optional Mode extension: a mode whose config
// section can bind per-application hotkey overrides.
//
// It exists to answer one question on the key path — whether resolving the
// focused app's bundle ID is worth the trip — so an implementation must stay a
// cheap read of config.
type hotkeyOverrideReporter interface {
	// HasAppHotkeyOverrides reports whether the mode's configuration binds any
	// per-application hotkey override.
	HasAppHotkeyOverrides() bool
}

// themeRefresher is an optional Mode extension: a mode with a themed drawing on
// screen that has to be put back in the colors the system just switched to.
//
// Hints, grid, recursive grid and the monitor picker each hold a Frame the new
// Style applies to. Scroll draws none of its own and idle has nothing up at all,
// so both carry this by not implementing it rather than by an empty arm.
//
// This is an effect rather than a getter, so a mode that does not carry it says
// so in the debug log (activeModeEffect): "the overlay stayed in the old theme"
// has to be answerable from a log rather than by reading the dispatch.
type themeRefresher interface {
	// RefreshForThemeChange draws the mode's Frame again so it picks the new
	// colors up. The overlay has already re-resolved every Style from the
	// configuration it holds, so the same Frame is all it takes.
	//
	// It reports whether the mode was in a state to redraw — false when it has
	// no session or nothing drawn to repaint. Whether the backend had a surface
	// to draw on is its own business and is reported in the log, not here.
	//
	// The caller holds h.mu across the whole dispatch, so the mode it selected
	// is still the active one here and an implementation must not re-check
	// (ADR 0004).
	RefreshForThemeChange() bool
}

// screenRefresher is an optional Mode extension: a mode whose drawing was built
// for the display configuration that has just changed underneath it, and so has
// to be built again rather than merely drawn again.
//
// Hints regenerates its collection from the accessibility tree, grid rebuilds
// its instance and recursive grid remaps its zoom history onto the new bounds.
// Scroll draws none of its own and the monitor picker places its panels per
// display, so neither carries this; idle has nothing on screen at all.
//
// This is an effect rather than a getter, so a mode that does not carry it says
// so in the debug log (activeModeEffect): "the overlay is still sized for the
// display I unplugged" has to be answerable from a log rather than by reading
// the dispatch.
type screenRefresher interface {
	// RefreshForScreenChange puts the mode back on the display as it now is.
	// The screen moved under it, so what it holds is rebuilt against the new
	// bounds and handed over as a transition — resized, shown and drawn.
	//
	// It reports whether the overlay was left sized for the new display, which
	// is what tells the caller it owes no resize of its own. That is not the
	// same as the refresh having succeeded: one that found nothing to draw and
	// left the mode still took the overlay off the display it was on, and
	// resizing it afterwards would bring it back up empty. False is for the
	// mode that rebuilt nothing at all — its feature switched off in
	// configuration, or no session state to rebuild from — because then the
	// overlay is still sized for the display that is gone.
	//
	// The caller holds h.mu across the whole dispatch, so the mode it selected
	// is still the active one here and an implementation must not re-check
	// (ADR 0004).
	RefreshForScreenChange(ctx context.Context) bool
}

// activeModeExtension resolves the active mode to the optional extension T,
// reporting false when no mode is active or when the active one does not carry
// that extension. It is the one place the comma-ok assertion is written, so a
// call site reads as "the mode that tracks a selection, if there is one".
//
// T is meant to be one of the extension interfaces above; instantiating it
// with anything else compiles and then matches nothing. It reads the mode map
// newModes builds, so a handler assembled without one reports every extension
// absent.
func activeModeExtension[T any](h *handlerState) (T, bool) {
	var absent T

	mode, exists := h.modes[h.appState.CurrentMode()]
	if !exists {
		return absent, false
	}

	extension, ok := mode.(T)
	if !ok {
		return absent, false
	}

	return extension, true
}

// activeModeEffect resolves the active mode to the optional extension T the way
// activeModeExtension does, and leaves a debug line naming the mode and the
// axis when the mode does not carry it.
//
// It is for the axes that perform an effect, never for a getter. A getter's
// absence is a zero value the caller was already prepared for; an effect's
// absence is a key the user pressed that did nothing, and this line is the
// difference between answering "why did backspace do nothing here?" from a log
// and answering it by reading the code. Idle takes the same line, with no mode
// registered to name: nothing was open to edit, which is the same nothing.
func activeModeEffect[T any](state *handlerState, axis extensionName) (T, bool) {
	extension, ok := activeModeExtension[T](state)
	if ok {
		return extension, true
	}

	state.logger.Debug(
		"Mode declined action",
		zap.String("mode", domain.ModeString(state.appState.CurrentMode())),
		zap.String("extension", string(axis)),
	)

	return extension, false
}
