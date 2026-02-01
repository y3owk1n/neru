package modes

import (
	"github.com/y3owk1n/neru/internal/core/domain"
)

// GridMode implements the Mode interface for grid-based navigation.
// It uses the generic mode implementation with grid-specific behavior.
type GridMode struct {
	*GenericMode
}

// NewGridMode creates a new grid mode implementation.
func NewGridMode(handler *Handler) *GridMode {
	return &GridMode{
		GenericMode: NewGenericMode(handler, domain.ModeGrid, "GridMode", ModeBehavior{}),
	}
}

// HintsMode implements the Mode interface for hints-based navigation.
// It uses the generic mode implementation with hints-specific behavior.
type HintsMode struct {
	*GenericMode
}

// NewHintsMode creates a new hints mode implementation.
func NewHintsMode(handler *Handler) *HintsMode {
	return &HintsMode{
		GenericMode: NewGenericMode(handler, domain.ModeHints, "HintsMode", ModeBehavior{}),
	}
}
