package modes

import "github.com/y3owk1n/neru/internal/core/domain"

const (
	// CursorSelectionModeFollow keeps the real cursor synced with the current selection.
	CursorSelectionModeFollow = "follow"
	// CursorSelectionModeHold keeps the real cursor stationary until explicit commit/move.
	CursorSelectionModeHold = "hold"
)

func resolveCursorFollowSelection(mode domain.Mode, override *bool) bool {
	if override != nil {
		return *override
	}

	switch mode {
	case domain.ModeHints, domain.ModeGrid, domain.ModeRecursiveGrid:
		return true
	case domain.ModeIdle, domain.ModeScroll:
		return false
	}

	return false
}
