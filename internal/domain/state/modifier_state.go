package state

import (
	"sync"

	"github.com/y3owk1n/neru/internal/domain/action"
)

// ModifierState manages the state of sticky modifiers in navigation modes.
// It tracks which modifier keys (Shift, Cmd, Alt, Ctrl) are currently toggled
// as sticky, allowing them to be applied to the next action.
type ModifierState struct {
	mu        sync.RWMutex
	modifiers action.Modifiers
}

// NewModifierState creates a new ModifierState with default values.
func NewModifierState() *ModifierState {
	return &ModifierState{}
}

// Toggle flips a single modifier bit and returns the new modifier state.
func (s *ModifierState) Toggle(mod action.Modifiers) action.Modifiers {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.modifiers ^= mod

	return s.modifiers
}

// Current returns the current set of sticky modifiers.
func (s *ModifierState) Current() action.Modifiers {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.modifiers
}

// Reset clears all sticky modifiers.
func (s *ModifierState) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.modifiers = 0
}
