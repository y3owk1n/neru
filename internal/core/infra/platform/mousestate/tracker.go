package mousestate

import (
	"image"
	"sync"

	"github.com/y3owk1n/neru/internal/core/domain/action"
)

// held records the state captured when a button was pressed.
type held struct {
	position  image.Point
	modifiers action.Modifiers
	down      bool
}

// Tracker records which mouse buttons are currently held down, together with
// the position and modifiers each button was pressed with. The zero value is
// ready to use and safe for concurrent use.
type Tracker struct {
	mu      sync.RWMutex
	buttons map[action.MouseButton]held
}

// SetDown records that button is held at position with the given modifiers.
func (t *Tracker) SetDown(
	button action.MouseButton,
	position image.Point,
	modifiers action.Modifiers,
) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.buttons == nil {
		t.buttons = make(map[action.MouseButton]held, len(action.MouseButtons()))
	}

	t.buttons[button] = held{position: position, modifiers: modifiers, down: true}
}

// Clear forgets any recorded state for button.
func (t *Tracker) Clear(button action.MouseButton) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.buttons, button)
}

// ClearAll forgets the recorded state for every button.
func (t *Tracker) ClearAll() {
	t.mu.Lock()
	defer t.mu.Unlock()

	clear(t.buttons)
}

// IsDown reports whether button is currently held.
func (t *Tracker) IsDown(button action.MouseButton) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.buttons[button].down
}

// AnyDown reports whether any button is currently held.
func (t *Tracker) AnyDown() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, state := range t.buttons {
		if state.down {
			return true
		}
	}

	return false
}

// HeldButtons returns the currently held buttons in left, right, middle order.
func (t *Tracker) HeldButtons() []action.MouseButton {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []action.MouseButton

	for _, button := range action.MouseButtons() {
		if t.buttons[button].down {
			result = append(result, button)
		}
	}

	return result
}

// DownPosition returns the position button was pressed at, and whether it is held.
func (t *Tracker) DownPosition(button action.MouseButton) (image.Point, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	state := t.buttons[button]

	return state.position, state.down
}

// DownModifiers returns the modifiers button was pressed with, and whether it is held.
func (t *Tracker) DownModifiers(button action.MouseButton) (action.Modifiers, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	state := t.buttons[button]

	return state.modifiers, state.down
}
