// Package textinput provides an adapter and stub for native text input.
package textinput

import (
	"context"
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/ports"
)

// Adapter implements the TextInputPort using the native TextInput.
type Adapter struct {
	input   *TextInput
	logger  *zap.Logger
	mu      sync.RWMutex
	running bool
}

// NewAdapter creates a new Adapter with the given TextInput and logger.
func NewAdapter(input *TextInput, logger *zap.Logger) *Adapter {
	return &Adapter{
		input:  input,
		logger: logger,
	}
}

// StartHintSearchSession starts the native hint search session.
func (a *Adapter) StartHintSearchSession(
	ctx context.Context,
	callbacks ports.TextInputCallbacks,
	frame ports.TextInputFrame,
) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.input == nil {
		return false, nil
	}

	started, err := a.input.StartHintSearchSession(ctx, callbacks, frame)
	if started {
		a.running = true
	}

	return started, err
}

// StopHintSearchSession stops the native hint search session.
func (a *Adapter) StopHintSearchSession(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.input == nil {
		return nil
	}

	a.running = false

	return a.input.StopHintSearchSession(ctx)
}
