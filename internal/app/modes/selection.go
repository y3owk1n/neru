package modes

import (
	"image"

	gridcomponent "github.com/y3owk1n/neru/internal/app/components/grid"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/core/domain"
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
	defer h.mu.Unlock()

	switch h.appState.CurrentMode() {
	case domain.ModeHints:
		if h.hints == nil || h.hints.Context == nil {
			return false, false
		}

		return h.hints.Context.ToggleCursorFollowSelection(), true
	case domain.ModeGrid:
		if h.grid == nil || h.grid.Context == nil {
			return false, false
		}

		enabled := h.grid.Context.ToggleCursorFollowSelection()
		h.refreshGridVirtualPointerLocked()

		return enabled, true
	case domain.ModeRecursiveGrid:
		if h.recursiveGrid == nil || h.recursiveGrid.Context == nil {
			return false, false
		}

		enabled := h.recursiveGrid.Context.ToggleCursorFollowSelection()
		h.refreshRecursiveGridVirtualPointerLocked()

		return enabled, true
	case domain.ModeIdle:
		return false, false
	case domain.ModeScroll:
		return false, false
	}

	return false, false
}

func (h *Handler) refreshGridVirtualPointerLocked() {
	if h.grid == nil || h.grid.Context == nil || h.grid.Overlay == nil {
		return
	}

	point, ok := h.grid.Context.SelectionPoint()
	if !ok || h.grid.Context.CursorFollowSelection() {
		h.grid.Overlay.HideVirtualPointer()

		return
	}

	h.grid.Overlay.ShowVirtualPointer(
		point,
		gridcomponent.BuildStyle(h.config.Grid, h.themeProvider),
	)
}

func (h *Handler) refreshRecursiveGridVirtualPointerLocked() {
	if h.recursiveGrid == nil || h.recursiveGrid.Context == nil || h.recursiveGrid.Overlay == nil {
		return
	}

	point, ok := h.recursiveGrid.Context.SelectionPoint()
	if !ok || h.recursiveGrid.Context.CursorFollowSelection() {
		h.recursiveGrid.Overlay.HideVirtualPointer()

		return
	}

	h.recursiveGrid.Overlay.ShowVirtualPointer(
		point,
		recursivegridcomponent.BuildStyle(h.config.RecursiveGrid, h.themeProvider),
	)
}
