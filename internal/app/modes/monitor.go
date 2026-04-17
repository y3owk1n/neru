package modes

import (
	"context"
	"image"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	domainHint "github.com/y3owk1n/neru/internal/core/domain/hint"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/ui/coordinates"
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
// `move_monitor <monitor-name>` instead.
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

	targetBounds, targetDisplayName, err := h.resolveMonitorTarget(ctx, direction)
	if err != nil {
		return err
	}

	center := image.Point{
		X: targetBounds.Min.X + targetBounds.Dx()/2,
		Y: targetBounds.Min.Y + targetBounds.Dy()/2,
	}

	// Hide the overlay *before* moving the cursor so the stale overlay on
	// the old monitor disappears immediately. Without this, the async
	// ResizeToActiveScreen (which reads [NSEvent mouseLocation] inside a
	// dispatch_async block) races the cursor warp: the overlay window may
	// still be visible on the old monitor while the new content is drawn,
	// or on rapid switching the resize picks up an intermediate mouse
	// position and targets the wrong display.
	//
	// The sequence Hide → cursor warp → Resize → Draw → Show ensures:
	//  1. The old overlay vanishes before the cursor moves.
	//  2. By the time ResizeToActiveScreen's dispatch_async block runs on
	//     the main queue, the cursor is already on the target monitor.
	//  3. Show() is dispatched last, so the overlay only becomes visible
	//     after both the resize and the redraw have been enqueued.
	hasActiveOverlay := h.appState.CurrentMode() != domain.ModeIdle
	if hasActiveOverlay && h.overlayManager != nil {
		h.overlayManager.Hide()
	}

	err = h.actionService.MoveCursorToPointAndWait(ctx, center, true)
	if err != nil {
		if hasActiveOverlay && h.overlayManager != nil {
			h.overlayManager.Show()
		}

		return err
	}

	h.logger.Info("Moved cursor to monitor",
		zap.String("monitor", targetDisplayName),
		zap.Int("x", center.X),
		zap.Int("y", center.Y),
	)

	h.refreshActiveModeOnNewScreen(ctx, targetBounds)

	return nil
}

// MoveMonitorByName moves the cursor to a specific monitor by name.
// If the mode overlay is active, it refreshes onto the new monitor.
func (h *Handler) MoveMonitorByName(
	ctx context.Context,
	monitorName string,
	offsetX, offsetY int,
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

	center := image.Point{
		X: bounds.Min.X + bounds.Dx()/2 + offsetX,
		Y: bounds.Min.Y + bounds.Dy()/2 + offsetY,
	}

	hasActiveOverlay := h.appState.CurrentMode() != domain.ModeIdle
	if hasActiveOverlay && h.overlayManager != nil {
		h.overlayManager.Hide()
	}

	err = h.actionService.MoveCursorToPointAndWait(ctx, center, true)
	if err != nil {
		if hasActiveOverlay && h.overlayManager != nil {
			h.overlayManager.Show()
		}

		return err
	}

	h.logger.Info("Moved cursor to monitor by name",
		zap.String("monitor", monitorName),
		zap.Int("x", center.X),
		zap.Int("y", center.Y),
	)

	h.refreshActiveModeOnNewScreen(ctx, bounds)

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

// refreshActiveModeOnNewScreen redraws the active mode overlay using the
// supplied target screen bounds. Unlike the lifecycle screen-change path
// (which re-queries the cursor position), this uses the bounds that
// MoveMonitor already resolved, eliminating the race between the Go-side
// ScreenBounds call and the native async ResizeToActiveScreen.
func (h *Handler) refreshActiveModeOnNewScreen(
	ctx context.Context,
	targetBounds image.Rectangle,
) {
	currentMode := h.appState.CurrentMode()
	if h.overlayManager == nil {
		return
	}

	switch currentMode {
	case domain.ModeGrid:
		h.overlayManager.ResizeToActiveScreen()
		h.refreshGridForMonitorMove(targetBounds)
	case domain.ModeRecursiveGrid:
		h.overlayManager.ResizeToActiveScreen()
		h.refreshRecursiveGridForMonitorMove(targetBounds)
	case domain.ModeHints:
		h.overlayManager.ResizeToActiveScreen()
		h.refreshHintsForMonitorMove(ctx, targetBounds)
	case domain.ModeScroll:
		h.overlayManager.ResizeToActiveScreen()
		h.overlayManager.Show()
	case domain.ModeIdle:
		return
	}
}

// refreshGridForMonitorMove regenerates the grid using the known target
// screen bounds and shows the overlay. Unlike RefreshGridForScreenChange
// this does not re-query ScreenBounds.
func (h *Handler) refreshGridForMonitorMove(targetBounds image.Rectangle) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.appState.CurrentMode() != domain.ModeGrid {
		return
	}
	// Use the known target bounds instead of re-querying ScreenBounds.
	h.screenBounds = targetBounds
	normalizedBounds := coordinates.NormalizeToLocalCoordinates(targetBounds)

	characters := h.config.Grid.Characters
	if strings.TrimSpace(characters) == "" {
		characters = h.config.Hints.HintCharacters
	}

	gridInstance := domainGrid.NewGridWithLabels(
		characters,
		h.config.Grid.RowLabels,
		h.config.Grid.ColLabels,
		normalizedBounds,
		h.logger,
	)
	h.grid.Context.SetGridInstanceValue(gridInstance)

	if h.grid.Manager != nil {
		h.grid.Manager.UpdateGrid(gridInstance)
		h.grid.Manager.Reset()
	}

	h.grid.Context.ClearSelectionPoint()

	drawGridErr := h.renderer.DrawGrid(gridInstance, "")
	if drawGridErr != nil {
		h.logger.Error("Failed to refresh grid after monitor move", zap.Error(drawGridErr))
		h.overlayManager.Show()

		return
	}

	h.refreshGridVirtualPointerLocked()
	h.overlayManager.Show()
}

// refreshRecursiveGridForMonitorMove remaps the recursive-grid to the known
// target screen bounds and shows the overlay. Unlike
// RefreshRecursiveGridForScreenChange this does not re-query ScreenBounds.
func (h *Handler) refreshRecursiveGridForMonitorMove(targetBounds image.Rectangle) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.appState.CurrentMode() != domain.ModeRecursiveGrid {
		return
	}

	h.screenBounds = targetBounds

	normalizedBounds := coordinates.NormalizeToLocalCoordinates(targetBounds)
	if h.recursiveGrid != nil && h.recursiveGrid.Manager != nil {
		h.recursiveGrid.Manager.CurrentGrid().RemapToNewBounds(normalizedBounds)
	} else {
		h.initializeRecursiveGridManager(normalizedBounds)
	}

	if h.recursiveGrid != nil && h.recursiveGrid.Context != nil {
		h.recursiveGrid.Context.ClearSelectionPoint()
	}

	h.updateRecursiveGridOverlay()
	h.refreshRecursiveGridVirtualPointerLocked()
	h.overlayManager.Show()
}

// refreshHintsForMonitorMove refreshes hints using the known target screen
// bounds. Unlike RefreshHintsForScreenChange this does not re-query
// ScreenBounds.
func (h *Handler) refreshHintsForMonitorMove(
	ctx context.Context,
	targetBounds image.Rectangle,
) {
	if h.hintService == nil {
		h.logger.Warn("Hint service unavailable after monitor move; exiting hints mode")
		h.ExitMode()

		return
	}

	domainHints, err := h.hintService.ShowHints(ctx)
	if err != nil {
		h.logger.Error(
			"Failed to refresh hints after monitor move",
			zap.Error(err),
		)
		h.ExitMode()

		return
	}

	if len(domainHints) == 0 {
		h.logger.Warn("No hints generated on target monitor; exiting hints mode")
		h.ExitMode()

		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.appState.CurrentMode() != domain.ModeHints {
		return
	}
	// Use the known target bounds instead of re-querying ScreenBounds.
	h.screenBounds = targetBounds

	filtered := filterHintsForScreen(domainHints, targetBounds)
	if len(filtered) == 0 {
		h.logger.Warn("All hints filtered out on target monitor; exiting hints mode")
		h.exitModeLocked()

		return
	}

	hintCollection := domainHint.NewCollection(filtered)
	h.hints.Context.SetHints(hintCollection)

	h.overlayManager.Show()
}
