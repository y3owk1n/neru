//go:build !darwin

package textinput

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/ports"
)

// TextInput is a stub implementation for platforms without native text input.
type TextInput struct {
	logger *zap.Logger
}

// NewTextInput creates a new stub TextInput instance.
func NewTextInput(logger *zap.Logger) *TextInput {
	return &TextInput{logger: logger}
}

// StartHintSearchSession does nothing on non-supported platforms and returns false.
func (t *TextInput) StartHintSearchSession(
	_ context.Context,
	_ ports.TextInputCallbacks,
	_ ports.TextInputFrame,
) (bool, error) {
	return false, nil
}

// StopHintSearchSession does nothing on non-supported platforms.
func (t *TextInput) StopHintSearchSession(_ context.Context) error {
	return nil
}
