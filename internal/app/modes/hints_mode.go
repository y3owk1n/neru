package modes

import (
	"github.com/y3owk1n/neru/internal/core/domain"
)

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
