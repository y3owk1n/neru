package modes

import (
	"image"

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

		return true
	case domain.ModeRecursiveGrid:
		if h.recursiveGrid == nil || h.recursiveGrid.Context == nil {
			return false
		}

		h.recursiveGrid.Context.ClearSelectionPoint()

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

		return h.grid.Context.ToggleCursorFollowSelection(), true
	case domain.ModeRecursiveGrid:
		if h.recursiveGrid == nil || h.recursiveGrid.Context == nil {
			return false, false
		}

		return h.recursiveGrid.Context.ToggleCursorFollowSelection(), true
	case domain.ModeIdle:
		return false, false
	case domain.ModeScroll:
		return false, false
	}

	return false, false
}
