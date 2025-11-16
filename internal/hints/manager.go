package hints

import (
	"strings"

	"go.uber.org/zap"
)

// Manager handles common hint input and filtering logic.
type Manager struct {
	currentInput string
	currentHints *HintCollection
	onHintUpdate func([]*Hint)
	logger       *zap.Logger
}

// NewManager creates a new hint manager.
func NewManager(onHintUpdate func([]*Hint), logger *zap.Logger) *Manager {
	return &Manager{
		onHintUpdate: onHintUpdate,
		logger:       logger,
	}
}

// SetHints sets the current hint collection.
func (m *Manager) SetHints(hints *HintCollection) {
	m.currentHints = hints
	m.currentInput = ""
	m.logger.Debug("Hint manager: Setting new hints", zap.Int("hint_count", len(hints.GetHints())))
	m.updateHints()
}

// Reset resets the current input and redraws all hints.
func (m *Manager) Reset() {
	m.currentInput = ""
	m.logger.Debug("Hint manager: Resetting input")
	m.updateHints()
}

// HandleInput processes an input character and returns true if an exact match was found.
func (m *Manager) HandleInput(key string) (*Hint, bool) {
	if m.currentHints == nil {
		m.logger.Debug("Hint manager: No current hints available")
		return nil, false
	}

	m.logger.Debug(
		"Hint manager: Processing input",
		zap.String("key", key),
		zap.String("current_input", m.currentInput),
	)

	// Handle backspace
	if key == "\x7f" || key == "delete" || key == "backspace" {
		if len(m.currentInput) > 0 {
			m.currentInput = m.currentInput[:len(m.currentInput)-1]
			m.logger.Debug(
				"Hint manager: Backspace processed",
				zap.String("new_input", m.currentInput),
			)
			m.updateHints()
		} else {
			m.logger.Debug("Hint manager: Resetting on backspace with empty input")
			m.Reset()
		}
		return nil, false
	}

	// Ignore non-letter keys
	if len(key) != 1 || !isLetter(key[0]) {
		m.logger.Debug("Hint manager: Ignoring non-letter key", zap.String("key", key))
		return nil, false
	}

	// Accumulate input (convert to uppercase to match hints)
	m.currentInput += strings.ToUpper(key)
	m.logger.Debug("Hint manager: Input accumulated", zap.String("current_input", m.currentInput))

	// Filter and update hints
	filtered := m.currentHints.FilterByPrefix(m.currentInput)
	m.logger.Debug("Hint manager: Filtered hints", zap.Int("filtered_count", len(filtered)))

	if len(filtered) == 0 {
		// No matches - reset
		m.logger.Debug("Hint manager: No matches found, resetting")
		m.currentInput = ""
		return nil, false
	}

	// Update matched prefix for filtered hints
	for _, hint := range filtered {
		hint.MatchedPrefix = m.currentInput
	}

	// Notify of hint updates
	m.onHintUpdate(filtered)

	// Check for exact match
	if len(filtered) == 1 && filtered[0].Label == m.currentInput {
		m.logger.Info("Hint manager: Exact match found", zap.String("label", filtered[0].Label))
		return filtered[0], true
	}

	return nil, false
}

// GetInput returns the current input string.
func (m *Manager) GetInput() string {
	return m.currentInput
}

// GetHints returns the current hints.
func (m *Manager) GetHints() []*Hint {
	return m.currentHints.GetHints()
}

// updateHints updates the hints based on the current input.
func (m *Manager) updateHints() {
	var filtered []*Hint
	if m.currentInput == "" {
		filtered = m.currentHints.GetHints()
		m.logger.Debug("Hint manager: Showing all hints", zap.Int("count", len(filtered)))
	} else {
		filtered = m.currentHints.FilterByPrefix(m.currentInput)
		m.logger.Debug("Hint manager: Showing filtered hints", zap.Int("count", len(filtered)), zap.String("prefix", m.currentInput))
	}

	// Update matched prefix for filtered hints
	for _, hint := range filtered {
		hint.MatchedPrefix = m.currentInput
	}

	// Notify of hint updates
	m.onHintUpdate(filtered)
}

func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
