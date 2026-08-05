package modes

import "github.com/y3owk1n/neru/internal/domain"

func resolveCursorFollowSelection(mode domain.Mode, override *bool) bool {
	if override != nil {
		return *override
	}

	switch mode {
	case domain.ModeHints, domain.ModeGrid, domain.ModeRecursiveGrid:
		return true
	case domain.ModeIdle, domain.ModeScroll, domain.ModeMonitorSelect:
		return false
	}

	return false
}
