package modes

import (
	"context"
	"image"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/geometry"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	domainHint "github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/ports"
)

// MonitorDirection selects how MoveMonitor picks the target monitor.
type MonitorDirection int

const (
	// MonitorDirectionNext advances forward through the monitor list.
	MonitorDirectionNext MonitorDirection = 1
	// MonitorDirectionPrevious steps backward through the monitor list.
	MonitorDirectionPrevious MonitorDirection = -1
)

// MoveMonitor moves the cursor to the center of the next (or previous)
// connected monitor and, when a mode overlay (hints/grid/recursive-grid) is
// active, refreshes it onto the new monitor.
//
// To jump to a specific named display, use
// `move_monitor --name <monitor-name>` instead.
func (h *Handler) MoveMonitor(
	ctx context.Context,
	direction MonitorDirection,
) error {
	// Serialize concurrent MoveMonitor calls. Rapid hotkey presses each
	// dispatch MoveMonitor in a fresh goroutine; without this lock a second
	// call can sample ScreenBounds mid-animation and race the first call's
	// overlay redraw, leaving the grid on the wrong monitor or half-drawn.
	h.moveMonitorMu.Lock()
	defer h.moveMonitorMu.Unlock()

	if h.system == nil {
		return derrors.New(derrors.CodeNotSupported, "system port unavailable")
	}

	if h.actionService == nil {
		return derrors.New(derrors.CodeActionFailed, "action service not available")
	}

	// "Next" is relative to the monitor under the cursor, and on Wayland the
	// cursor position is a cache a user-driven mouse move invalidates — refresh
	// it first or the step starts from the last place Neru put the cursor, not
	// where the user did (#1279). Runs outside h.mu, like the warp below.
	h.syncCursorPosition(ctx)

	targetBounds, targetDisplayName, err := h.resolveMonitorTarget(ctx, direction)
	if err != nil {
		return err
	}

	return h.moveCursorToMonitor(ctx, targetBounds, targetDisplayName)
}

// MoveMonitorByName moves the cursor to a specific monitor by name.
// If the mode overlay is active, it refreshes onto the new monitor.
func (h *Handler) MoveMonitorByName(
	ctx context.Context,
	monitorName string,
) error {
	h.moveMonitorMu.Lock()
	defer h.moveMonitorMu.Unlock()

	if h.system == nil {
		return derrors.New(derrors.CodeNotSupported, "system port unavailable")
	}

	if h.actionService == nil {
		return derrors.New(derrors.CodeActionFailed, "action service not available")
	}

	names, err := h.system.ScreenNames(ctx)
	if err != nil {
		return err
	}

	if len(names) == 0 {
		return derrors.New(derrors.CodeInvalidInput, "no monitors detected")
	}

	bounds, found, err := h.system.ScreenBoundsByName(ctx, monitorName)
	if err != nil {
		return err
	}

	if !found {
		return derrors.Newf(
			derrors.CodeInvalidInput,
			"monitor not found: %s, available: %s",
			monitorName,
			strings.Join(names, ", "),
		)
	}

	return h.moveCursorToMonitor(ctx, bounds, monitorName)
}

// moveCursorToMonitor takes the active mode's overlay off the screen, warps the
// cursor to the center of the display given, and puts the mode back on the
// display it lands on.
//
// Clear → warp → redraw, in that order: sizing the overlay to a screen reads
// the mouse position on the main queue, so taking the frame off first removes
// the stale overlay before the warp, and everything that puts it back runs
// with the cursor already on the target monitor. Putting it back is one Frame
// per mode, and the overlay owns the resize/show/draw order inside it. A warp
// that fails puts the mode back on the display it never left.
func (h *Handler) moveCursorToMonitor(
	ctx context.Context,
	bounds image.Rectangle,
	monitorName string,
) error {
	center := image.Point{
		X: bounds.Min.X + bounds.Dx()/2,
		Y: bounds.Min.Y + bounds.Dy()/2,
	}

	sourceBounds, hasActiveOverlay := h.clearFrameForMonitorMove()

	err := h.actionService.MoveCursorToPointAndWait(ctx, center, true)
	if err != nil {
		if hasActiveOverlay {
			h.refreshActiveModeForMonitorMove(ctx, sourceBounds)
		}

		return err
	}

	h.logger.Debug("Moved cursor to monitor",
		zap.String("monitor", monitorName),
		zap.Int("x", center.X),
		zap.Int("y", center.Y),
	)

	h.refreshActiveModeForMonitorMove(ctx, bounds)

	return nil
}

// resolveMonitorTarget returns the bounds and display name of the next monitor
// in the requested direction relative to the one currently under the cursor.
func (h *Handler) resolveMonitorTarget(
	ctx context.Context,
	direction MonitorDirection,
) (image.Rectangle, string, error) {
	names, err := h.system.ScreenNames(ctx)
	if err != nil {
		return image.Rectangle{}, "", err
	}

	if len(names) == 0 {
		return image.Rectangle{}, "", derrors.New(
			derrors.CodeInvalidInput,
			"no monitors detected",
		)
	}

	if len(names) == 1 {
		return image.Rectangle{}, "", derrors.New(
			derrors.CodeInvalidInput,
			"only one monitor detected; move_monitor requires at least two",
		)
	}

	active, err := h.system.ScreenBounds(ctx)
	if err != nil {
		return image.Rectangle{}, "", err
	}

	step := int(direction)
	if step == 0 {
		step = int(MonitorDirectionNext)
	}

	currentIdx := indexOfScreen(ctx, h.system, names, active)

	nextIdx := ((currentIdx+step)%len(names) + len(names)) % len(names)
	nextName := names[nextIdx]

	bounds, found, err := h.system.ScreenBoundsByName(ctx, nextName)
	if err != nil {
		return image.Rectangle{}, "", err
	}

	if !found {
		return image.Rectangle{}, "", derrors.Newf(
			derrors.CodeInvalidInput,
			"monitor not found: %s",
			nextName,
		)
	}

	return bounds, nextName, nil
}

// indexOfScreen returns the index of the monitor whose bounds equal active, or
// 0 when no match is found (so "next" advances past the first entry).
func indexOfScreen(
	ctx context.Context,
	system interface {
		ScreenBoundsByName(
			ctx context.Context,
			name string,
		) (image.Rectangle, bool, error)
	},
	names []string,
	active image.Rectangle,
) int {
	for idx, name := range names {
		bounds, found, err := system.ScreenBoundsByName(ctx, name)
		if err != nil || !found {
			continue
		}

		if bounds == active {
			return idx
		}
	}

	return 0
}

// clearFrameForMonitorMove takes the active mode's overlay off the screen
// before the cursor warps, so the drawing does not linger on the monitor being
// left. What puts it back is the redraw on the other side.
//
// It reports the display the mode was drawn against, and whether there was a
// mode drawn at all, from the same locked section that clears: read separately,
// a mode that exited in between would leave the caller restoring a frame that
// no longer exists.
//
// The clear reaches the overlay under h.mu, which on macOS makes a synchronous
// hop to the main queue when monitor-select panels are on screen. That is safe
// only while nothing running on the main queue takes h.mu — the invariant the
// darwin overlay already states — and this is the third path that leans on it
// (`internal/app/modes/AGENTS.md`).
func (h *Handler) clearFrameForMonitorMove() (image.Rectangle, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.appState.CurrentMode() == domain.ModeIdle {
		return image.Rectangle{}, false
	}

	h.clearOverlayFrameForRedraw()

	return h.screenBounds, true
}

// refreshActiveModeForMonitorMove puts the active mode back on screen against
// the supplied screen bounds. Unlike the lifecycle screen-change path (which
// re-queries the cursor position), this uses the bounds that MoveMonitor
// already resolved, eliminating the race between the Go-side ScreenBounds call
// and the backend's async resize.
//
// Every mode comes back the way it came up in the first place: as a Frame,
// with the overlay owning the resize, show and draw inside it.
//
// This is the whole dispatch and it runs under one hold of h.mu: the mode is
// read once, selected once and called once, so it cannot change between being
// chosen and being used and no implementation re-checks it (ADR 0004). Idle
// has no entry in the mode map, which is what makes a monitor move with
// nothing open draw nothing.
//
// The lock is taken here rather than in MoveMonitor because the animated
// cursor warp runs before this call and must stay outside the hold; the
// blocking work that remains inside it — hint regeneration — is the same work
// the activation and screen-change paths already do under the lock.
func (h *Handler) refreshActiveModeForMonitorMove(
	ctx context.Context,
	targetBounds image.Rectangle,
) {
	h.mu.Lock()
	defer h.mu.Unlock()

	mode, exists := h.modes[h.appState.CurrentMode()]
	if !exists {
		return
	}

	mode.RefreshForMonitorMove(ctx, targetBounds)
}

// refreshScrollForMonitorMove puts scroll mode back on the overlay after a
// monitor move. It draws nothing, but the overlay has to be switched back to
// the mode the indicators name.
func (h *handlerState) refreshScrollForMonitorMove() {
	h.showFrame(ports.ScrollFrame{}, "refresh scroll after monitor move")
}

// refreshMonitorSelectForMonitorMove redraws the monitor picker after a
// monitor move. The panels are placed per display, so they come back exactly
// as they were — but they were taken off the screen with the frame, and
// leaving them off would strand the user in a mode with nothing to pick from.
func (h *handlerState) refreshMonitorSelectForMonitorMove() {
	if h.monitorSelect == nil {
		return
	}

	h.showFrame(h.monitorSelectFrame(), "refresh monitor_select after monitor move")
}

// refreshGridForMonitorMove regenerates the grid using the known target
// screen bounds and shows the overlay. Unlike refreshGridForScreenChange
// this does not re-query ScreenBounds.
func (h *handlerState) refreshGridForMonitorMove(targetBounds image.Rectangle) {
	// Use the known target bounds instead of re-querying ScreenBounds.
	h.setScreenBounds(targetBounds)
	normalizedBounds := geometry.NormalizeToLocalCoordinates(targetBounds)

	gridInstance := domainGrid.NewGridWithOptions(
		h.config.GridOptions(), normalizedBounds, h.logger,
	)
	h.grid.Context.SetGridInstanceValue(gridInstance)

	if h.grid.Manager != nil {
		h.grid.Manager.UpdateGrid(gridInstance)
		h.grid.Manager.Reset()
	}

	h.grid.Context.ClearSelectionPoint()

	// The grid moved to another display, so it comes up there the same way it
	// came up here: as a Frame, resized and shown by the overlay.
	if !h.showFrame(ports.GridFrame{Grid: gridInstance}, "refresh grid after monitor move") {
		return
	}

	h.refreshGridVirtualPointer()
}

// refreshRecursiveGridForMonitorMove remaps the recursive-grid to the known
// target screen bounds and shows the overlay. Unlike
// refreshRecursiveGridForScreenChange this does not re-query ScreenBounds.
func (h *handlerState) refreshRecursiveGridForMonitorMove(targetBounds image.Rectangle) {
	h.setScreenBounds(targetBounds)

	normalizedBounds := geometry.NormalizeToLocalCoordinates(targetBounds)
	if h.recursiveGrid != nil && h.recursiveGrid.Manager != nil {
		h.recursiveGrid.Manager.CurrentGrid().RemapToNewBounds(normalizedBounds)
	} else {
		h.initializeRecursiveGridManager(normalizedBounds)
	}

	if h.recursiveGrid != nil && h.recursiveGrid.Context != nil {
		h.recursiveGrid.Context.ClearSelectionPoint()
	}

	// Same as grid: the recursive grid comes up on the new display as a Frame.
	if !h.showFrame(h.recursiveGridFrame(), "refresh recursive-grid after monitor move") {
		return
	}

	h.refreshRecursiveGridVirtualPointer()
}

// refreshHintsForMonitorMove refreshes hints using the known target screen
// bounds. Unlike refreshHintsForScreenChange this does not re-query
// ScreenBounds.
//
// The whole of it — including the accessibility-tree walk GenerateHints does —
// runs under h.mu, which is where the activation path and the screen-change
// path already do the same work. What it buys is that the flags the session
// was activated with are read under the same lock that writes them, rather
// than concurrently with a mode exit clearing them.
//
// The walk is given the same HintTimeout budget the activation path gives it.
// The context a move_monitor step arrives with carries no deadline of its own,
// and a tree that never answers would pin h.mu — and with it every keystroke —
// for as long as it took.
func (h *handlerState) refreshHintsForMonitorMove(
	ctx context.Context,
	targetBounds image.Rectangle,
) {
	if h.hintService == nil {
		h.logger.Warn("Hint service unavailable after monitor move; exiting hints mode")
		h.exitMode()

		return
	}

	generateCtx, cancelGenerate := context.WithTimeout(ctx, HintTimeout)
	defer cancelGenerate()

	filterRoles := h.hints.Context.FilterRoles()
	filterTextContains := h.hints.Context.FilterTextContains()
	strategyOverride := h.hints.Context.StrategyOverride()
	captureScopeOverride := h.hints.Context.CaptureScopeOverride()
	labelDirectionOverride := h.hints.Context.LabelDirectionOverride()

	splitWordOverride := false
	if h.hints != nil && h.hints.Context != nil {
		splitWordOverride = h.hints.Context.SplitWord()
	}

	domainHints, err := h.hintService.GenerateHints(
		generateCtx,
		filterRoles,
		filterTextContains,
		"",
		strategyOverride,
		captureScopeOverride,
		labelDirectionOverride,
		splitWordOverride,
	)
	if err != nil {
		h.logger.Error(
			"Failed to refresh hints after monitor move",
			zap.Error(err),
		)
		h.exitMode()

		return
	}

	if len(domainHints) == 0 {
		h.logger.Warn("No hints generated on target monitor; exiting hints mode")
		h.exitMode()

		return
	}

	// Use the known target bounds instead of re-querying ScreenBounds.
	h.setScreenBounds(targetBounds)

	filtered := filterHintsForScreen(domainHints, targetBounds)
	if len(filtered) == 0 {
		h.logger.Warn("All hints filtered out on target monitor; exiting hints mode")
		h.exitMode()

		return
	}

	hintCollection := domainHint.NewCollection(filtered)

	// The frame came off the screen before the warp, so this draw performs the
	// window sequence again. As on the activation path, the flag is cleared
	// immediately before SetHints — which bumps the hint manager's update
	// generation in the same locked section, so a debounce timer that fired
	// during the move cannot re-show the overlay on the display just left.
	h.hintsFrameOnScreen = false

	setHintsErr := h.hints.Context.SetHints(hintCollection)
	if setHintsErr != nil {
		h.logger.Error("Failed to set hints after monitor move", zap.Error(setHintsErr))
		h.exitMode()

		return
	}
	// SetHints fires the hint manager's update callback, which hands the
	// labels over as a Frame — and since the frame was cleared before the
	// warp, that first draw is the transition that brings the overlay back up.
}
