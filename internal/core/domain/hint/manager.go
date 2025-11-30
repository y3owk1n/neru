package hint

import (
	"strings"
	"time"

	"github.com/y3owk1n/neru/internal/core/domain"
	"go.uber.org/zap"
)

// Manager handles hint generation and management.
type Manager struct {
	domain.BaseManager

	hints            *Collection
	onUpdate         func([]*Interface) // Callback when filtered hints change
	debounceTimer    *time.Timer
	debounceDuration time.Duration
}

const (
	// DefaultDebounceDuration is the default debounce duration for hint updates.
	DefaultDebounceDuration = 50 * time.Millisecond
)

// NewManager creates a new hint manager with the specified logger.
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		BaseManager: domain.BaseManager{
			Logger: logger,
		},
		debounceDuration: DefaultDebounceDuration,
	}
}

// SetUpdateCallback sets the callback function to be called when filtered hints change.
func (m *Manager) SetUpdateCallback(callback func([]*Interface)) {
	m.onUpdate = callback
}

// SetHints updates the current hint collection and resets the input state.
func (m *Manager) SetHints(hints *Collection) {
	// Cancel any pending debounced updates
	if m.debounceTimer != nil {
		m.debounceTimer.Stop()
		m.debounceTimer = nil
	}

	m.hints = hints
	m.SetCurrentInput("")

	// Trigger immediate update callback with all hints on initial set
	if m.onUpdate != nil && hints != nil {
		m.onUpdate(hints.All())
	}
}

// Reset clears the current input.
func (m *Manager) Reset() {
	// Cancel any pending debounced updates
	if m.debounceTimer != nil {
		m.debounceTimer.Stop()
		m.debounceTimer = nil
	}

	m.SetCurrentInput("")
	// Trigger immediate update callback with all hints
	if m.onUpdate != nil && m.hints != nil {
		m.onUpdate(m.hints.All())
	}
}

// HandleInput processes an input character and returns the matched hint if an exact match is found.
// Handles backspace for input correction, filters hints by prefix, and detects exact matches.
// Returns (hint, true) if exact match found, (nil, false) otherwise.
// Maintains input state and triggers overlay updates for filtered hints.
func (m *Manager) HandleInput(key string) (*Interface, bool) {
	if m.hints == nil {
		return nil, false
	}

	if m.Logger != nil {
		m.Logger.Debug("Hint manager: Processing input",
			zap.String("key", key),
			zap.String("current_input", m.CurrentInput()))
	}

	// Handle backspace to allow input correction
	if key == "\x7f" || key == "delete" || key == "backspace" {
		if len(m.CurrentInput()) > 0 {
			m.SetCurrentInput(m.CurrentInput()[:len(m.CurrentInput())-1])

			// Update overlay to show filtered hints with new prefix
			if m.hints != nil {
				filtered := m.hints.FilterByPrefix(m.CurrentInput())

				hintsWithPrefix := make([]*Interface, len(filtered))
				for i, h := range filtered {
					hintsWithPrefix[i] = h.WithMatchedPrefix(m.CurrentInput())
				}

				m.debouncedUpdate(hintsWithPrefix)
			}
		} else {
			// Reset to show all hints if backspacing from empty input
			m.Reset()
		}

		return nil, false
	}

	// Ignore non-letter keys
	if len(key) != 1 || !isLetter(key[0]) {
		return nil, false
	}

	// Accumulate input (convert to uppercase to match hints)
	m.SetCurrentInput(m.CurrentInput() + strings.ToUpper(key))

	// Filter hints by prefix
	filtered := m.hints.FilterByPrefix(m.CurrentInput())
	if m.Logger != nil {
		m.Logger.Debug("Hint manager: Filtered hints", zap.Int("filtered_count", len(filtered)))
	}

	if len(filtered) == 0 {
		// No matches - reset
		m.SetCurrentInput("")

		return nil, false
	}

	// Update matched prefix for all filtered hints
	hintsWithPrefix := make([]*Interface, len(filtered))
	for i, h := range filtered {
		hintsWithPrefix[i] = h.WithMatchedPrefix(m.CurrentInput())
	}

	// Check for exact match
	if len(hintsWithPrefix) == 1 && hintsWithPrefix[0].Label() == m.CurrentInput() {
		if m.Logger != nil {
			m.Logger.Info("Hint manager: Exact match found",
				zap.String("label", hintsWithPrefix[0].Label()))
		}

		return hintsWithPrefix[0], true
	}

	// Notify update callback with filtered hints (with matched prefix set)
	m.debouncedUpdate(hintsWithPrefix)

	return nil, false
}

// FilteredHints returns hints filtered by the current input.
func (m *Manager) FilteredHints() []*Interface {
	if m.hints == nil {
		return nil
	}

	if m.CurrentInput() == "" {
		return m.hints.All()
	}

	return m.hints.FilterByPrefix(m.CurrentInput())
}

// debouncedUpdate schedules a debounced update of the overlay.
func (m *Manager) debouncedUpdate(hints []*Interface) {
	// Cancel any existing timer
	if m.debounceTimer != nil {
		m.debounceTimer.Stop()
	}

	// Start new timer
	m.debounceTimer = time.AfterFunc(m.debounceDuration, func() {
		if m.onUpdate != nil {
			m.onUpdate(hints)
		}
	})
}

// isLetter checks if a byte is a letter.
func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
