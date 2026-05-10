//go:build !darwin

package textinput

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/ports"
)

type TextInput struct {
	logger *zap.Logger
}

func NewTextInput(logger *zap.Logger) *TextInput {
	return &TextInput{logger: logger}
}

func (t *TextInput) StartHintSearchSession(
	_ context.Context,
	_ ports.TextInputCallbacks,
	_ ports.TextInputFrame,
) (bool, error) {
	return false, nil
}

func (t *TextInput) StopHintSearchSession(_ context.Context) error {
	return nil
}
