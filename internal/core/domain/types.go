package domain

import "go.uber.org/zap"

// BaseManager provides common functionality for domain managers.
// It contains shared fields and methods used across different domain managers.
type BaseManager struct {
	currentInput string
	Logger       *zap.Logger
}

// SetCurrentInput sets the current input string.
func (m *BaseManager) SetCurrentInput(input string) {
	m.currentInput = input
}

// CurrentInput returns the current input string.
func (m *BaseManager) CurrentInput() string {
	return m.currentInput
}

// Reset resets the base manager to its initial state.
func (m *BaseManager) Reset() {
	m.currentInput = ""
}
