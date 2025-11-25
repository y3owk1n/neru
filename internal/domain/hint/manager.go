package hint

import (
	"strings"

	"go.uber.org/zap"
)

// Manager handles hint generation and management.
type Manager struct {
	currentInput string
	hints        *Collection
	onUpdate     func([]*Interface) // Callback when filtered hints change
	logger       *zap.Logger
}

// NewManager creates a new hint manager with the specified logger.
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		logger: logger,
	}
}

// SetUpdateCallback sets the callback function to be called when filtered hints change.
func (m *Manager) SetUpdateCallback(callback func([]*Interface)) {
	m.onUpdate = callback
}

// SetHints updates the current hint collection and resets the input state.
func (m *Manager) SetHints(hints *Collection) {
	m.hints = hints

	m.currentInput = ""
	// Trigger update callback with all hints on initial set
	if m.onUpdate != nil && hints != nil {
		m.onUpdate(hints.All())
	}
}

// Reset clears the current input.
func (m *Manager) Reset() {
	m.currentInput = ""
	// Trigger update callback with all hints
	if m.onUpdate != nil && m.hints != nil {
		m.onUpdate(m.hints.All())
	}
}

// HandleInput processes an input character and returns the matched hint if an exact match is found.
// Returns (hint, true) if exact match found, (nil, false) otherwise.
func (m *Manager) HandleInput(key string) (*Interface, bool) {
	if m.hints == nil {
		return nil, false
	}

	if m.logger != nil {
		m.logger.Debug("Hint manager: Processing input",
			zap.String("key", key),
			zap.String("current_input", m.currentInput))
	}

	// Handle backspace
	if key == "\x7f" || key == "delete" || key == "backspace" {
		if len(m.currentInput) > 0 {
			m.currentInput = m.currentInput[:len(m.currentInput)-1]

			// Update view for backspace
			if m.hints != nil {
				filtered := m.hints.FilterByPrefix(m.currentInput)

				hintsWithPrefix := make([]*Interface, len(filtered))
				for i, h := range filtered {
					hintsWithPrefix[i] = h.WithMatchedPrefix(m.currentInput)
				}

				if m.onUpdate != nil {
					m.onUpdate(hintsWithPrefix)
				}
			}
		} else {
			m.Reset()
		}

		return nil, false
	}

	// Ignore non-letter keys
	if len(key) != 1 || !isLetter(key[0]) {
		return nil, false
	}

	// Accumulate input (convert to uppercase to match hints)
	m.currentInput += strings.ToUpper(key)

	// Filter hints by prefix
	filtered := m.hints.FilterByPrefix(m.currentInput)
	if m.logger != nil {
		m.logger.Debug("Hint manager: Filtered hints", zap.Int("filtered_count", len(filtered)))
	}

	if len(filtered) == 0 {
		// No matches - reset
		m.currentInput = ""

		return nil, false
	}

	// Update matched prefix for all filtered hints
	hintsWithPrefix := make([]*Interface, len(filtered))
	for i, h := range filtered {
		hintsWithPrefix[i] = h.WithMatchedPrefix(m.currentInput)
	}

	// Check for exact match
	if len(hintsWithPrefix) == 1 && hintsWithPrefix[0].Label() == m.currentInput {
		if m.logger != nil {
			m.logger.Info("Hint manager: Exact match found",
				zap.String("label", hintsWithPrefix[0].Label()))
		}

		return hintsWithPrefix[0], true
	}

	// Notify update callback with filtered hints (with matched prefix set)
	if m.onUpdate != nil {
		m.onUpdate(hintsWithPrefix)
	}

	return nil, false
}

// GetInput returns the current input string.
func (m *Manager) GetInput() string {
	return m.currentInput
}

// GetFilteredHints returns hints filtered by the current input.
func (m *Manager) GetFilteredHints() []*Interface {
	if m.hints == nil {
		return nil
	}

	if m.currentInput == "" {
		return m.hints.All()
	}

	return m.hints.FilterByPrefix(m.currentInput)
}

// isLetter checks if a byte is a letter.
func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
