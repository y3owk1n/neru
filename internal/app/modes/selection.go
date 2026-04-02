package modes

import (
	"context"
	"image"

	"go.uber.org/zap"

	componentrecursivegrid "github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/ui/coordinates"
)

// CurrentSelectionPoint returns the active selection point for the current mode, if any.
func (h *Handler) CurrentSelectionPoint() (image.Point, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch h.appState.CurrentMode() {
	case domain.ModeIdle:
		return image.Point{}, false
	case domain.ModeHints:
		return image.Point{}, false
	case domain.ModeGrid:
		if h.grid == nil || h.grid.Context == nil {
			return image.Point{}, false
		}

		return h.grid.Context.SelectionPoint()
	case domain.ModeRecursiveGrid:
		if h.recursiveGrid == nil || h.recursiveGrid.Context == nil {
			return image.Point{}, false
		}

		return h.recursiveGrid.Context.SelectionPoint()
	case domain.ModeScroll:
		return image.Point{}, false
	}

	return image.Point{}, false
}

// ClearCurrentSelectionPoint removes the active selection point for the current mode.
func (h *Handler) ClearCurrentSelectionPoint() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch h.appState.CurrentMode() {
	case domain.ModeGrid:
		if h.grid == nil || h.grid.Context == nil {
			return false
		}

		h.grid.Context.ClearSelectionPoint()
		h.refreshGridVirtualPointerLocked()

		return true
	case domain.ModeRecursiveGrid:
		if h.recursiveGrid == nil || h.recursiveGrid.Context == nil {
			return false
		}

		h.recursiveGrid.Context.ClearSelectionPoint()
		h.refreshRecursiveGridVirtualPointerLocked()

		return true
	case domain.ModeHints:
		return false
	case domain.ModeIdle:
		return false
	case domain.ModeScroll:
		return false
	}

	return false
}

// ToggleCursorFollowSelection toggles cursor-follow-selection for the active mode.
func (h *Handler) ToggleCursorFollowSelection() (bool, bool) {
	h.mu.Lock()

	var (
		enabled    bool
		supported  bool
		target     image.Point
		shouldMove bool
	)

	switch h.appState.CurrentMode() {
	case domain.ModeHints:
		if h.hints == nil || h.hints.Context == nil {
			h.mu.Unlock()

			return false, false
		}

		enabled = h.hints.Context.ToggleCursorFollowSelection()
		supported = true
	case domain.ModeGrid:
		if h.grid == nil || h.grid.Context == nil {
			h.mu.Unlock()

			return false, false
		}

		enabled = h.grid.Context.ToggleCursorFollowSelection()
		if enabled {
			target, shouldMove = h.grid.Context.SelectionPoint()
		}

		h.refreshGridVirtualPointerLocked()

		supported = true

	case domain.ModeRecursiveGrid:
		if h.recursiveGrid == nil || h.recursiveGrid.Context == nil {
			h.mu.Unlock()

			return false, false
		}

		enabled = h.recursiveGrid.Context.ToggleCursorFollowSelection()
		if enabled {
			target, shouldMove = h.recursiveGrid.Context.SelectionPoint()
		}

		h.refreshRecursiveGridVirtualPointerLocked()

		supported = true

	case domain.ModeIdle:
		h.mu.Unlock()

		return false, false
	case domain.ModeScroll:
		h.mu.Unlock()

		return false, false
	}

	h.mu.Unlock()

	if enabled && shouldMove && h.actionService != nil {
		moveCursorErr := h.actionService.MoveCursorToPoint(context.Background(), target)
		if moveCursorErr != nil {
			h.logger.Error("Failed to move cursor", zap.Error(moveCursorErr))
		}
	}

	return enabled, supported
}

func (h *Handler) refreshGridVirtualPointerLocked() {
	if h.grid == nil || h.grid.Context == nil || h.grid.Overlay == nil {
		return
	}

	point, ok := h.grid.Context.SelectionPoint()

	size, fillColor, enabled := h.virtualPointerStyle()
	if !ok || h.grid.Context.CursorFollowSelection() || !enabled {
		h.grid.Overlay.HideVirtualPointer()

		return
	}

	localPoint := coordinates.ConvertToLocalCoordinates(point, h.screenBounds)
	h.grid.Overlay.ShowVirtualPointer(localPoint, size, fillColor)
}

func (h *Handler) refreshRecursiveGridVirtualPointerLocked() {
	if h.recursiveGrid == nil || h.recursiveGrid.Context == nil || h.recursiveGrid.Overlay == nil {
		return
	}

	state := h.currentRecursiveGridVirtualPointerState()
	if !state.Visible {
		h.recursiveGrid.Overlay.HideVirtualPointer()

		return
	}

	h.recursiveGrid.Overlay.ShowVirtualPointer(state.Position, state.Size, state.FillColor)
}

func (h *Handler) currentRecursiveGridVirtualPointerState() componentrecursivegrid.VirtualPointerState {
	if h.recursiveGrid == nil || h.recursiveGrid.Context == nil {
		return componentrecursivegrid.VirtualPointerState{}
	}

	point, ok := h.recursiveGrid.Context.SelectionPoint()

	size, fillColor, enabled := h.virtualPointerStyle()
	if !ok || h.recursiveGrid.Context.CursorFollowSelection() || !enabled {
		return componentrecursivegrid.VirtualPointerState{}
	}

	return componentrecursivegrid.VirtualPointerState{
		Visible:   true,
		Position:  coordinates.ConvertToLocalCoordinates(point, h.screenBounds),
		Size:      size,
		FillColor: fillColor,
	}
}

func (h *Handler) virtualPointerStyle() (int, string, bool) {
	cfg := h.config.VirtualPointer
	if !cfg.Enabled {
		return 0, "", false
	}

	fillColor := cfg.UI.Color.ForTheme(
		h.themeProvider,
		config.VirtualPointerColorLight,
		config.VirtualPointerColorDark,
	)

	return cfg.UI.Size, fillColor, true
}
