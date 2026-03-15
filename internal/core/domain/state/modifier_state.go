package state

import (
	"sync"

	"github.com/y3owk1n/neru/internal/core/domain/action"
)

// ModifierState manages the state of sticky modifiers in navigation modes.
// It tracks which modifier keys (Shift, Cmd, Alt, Ctrl) are currently toggled
// as sticky, allowing them to be applied to the next action.
type ModifierState struct {
	mu             sync.RWMutex
	modifiers      action.Modifiers
	callbacks      map[uint64]func(action.Modifiers)
	nextCallbackID uint64
}

// NewModifierState creates a new ModifierState with default values.
func NewModifierState() *ModifierState {
	return &ModifierState{
		callbacks:      make(map[uint64]func(action.Modifiers)),
		nextCallbackID: 1,
	}
}

// Toggle flips a single modifier bit and notifies subscribers.
// Returns the new modifier state after the toggle.
func (s *ModifierState) Toggle(mod action.Modifiers) action.Modifiers {
	s.mu.Lock()
	s.modifiers ^= mod
	newModifiers := s.modifiers

	callbacks := make([]func(action.Modifiers), 0, len(s.callbacks))
	for _, cb := range s.callbacks {
		callbacks = append(callbacks, cb)
	}
	s.mu.Unlock()

	for _, callback := range callbacks {
		if callback != nil {
			callback(newModifiers)
		}
	}

	return newModifiers
}

// Current returns the current set of sticky modifiers.
func (s *ModifierState) Current() action.Modifiers {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.modifiers
}

// Reset clears all sticky modifiers. Notifies subscribers only if the state changed.
func (s *ModifierState) Reset() {
	s.mu.Lock()
	oldModifiers := s.modifiers
	s.modifiers = 0

	if oldModifiers == 0 {
		s.mu.Unlock()

		return
	}

	callbacks := make([]func(action.Modifiers), 0, len(s.callbacks))
	for _, cb := range s.callbacks {
		callbacks = append(callbacks, cb)
	}
	s.mu.Unlock()

	for _, callback := range callbacks {
		if callback != nil {
			callback(0)
		}
	}
}

// OnChange registers a callback for when the modifier state changes.
// Returns a subscription ID that can be used to unsubscribe later.
func (s *ModifierState) OnChange(callback func(action.Modifiers)) uint64 {
	if callback == nil {
		return 0
	}

	s.mu.Lock()
	subscriptionID := s.nextCallbackID
	s.nextCallbackID++
	s.callbacks[subscriptionID] = callback
	currentModifiers := s.modifiers
	s.mu.Unlock()

	go callback(currentModifiers)

	return subscriptionID
}

// OffChange unsubscribes a callback using the ID returned by OnChange.
func (s *ModifierState) OffChange(subscriptionID uint64) {
	s.mu.Lock()
	delete(s.callbacks, subscriptionID)
	s.mu.Unlock()
}
