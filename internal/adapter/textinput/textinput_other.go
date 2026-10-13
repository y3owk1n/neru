//go:build !darwin && !linux

package textinput

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/ports"
)

// TextInput is the slot for platforms with no native hint-search field,
// Windows among them. Hint search reads the event tap's key stream there
// instead.
//
// The started == false return — rather than a CodeNotSupported error — is the
// contract here: TextInputPort.StartHintSearchSession is defined as
// best-effort, and callers already branch on started to pick the key-stream
// path. Reporting an error would make an expected degrade look like a failure.
// The unavailability is instead surfaced through the text_input entry in
// ports.PlatformCapabilities, which reports stub here.
type TextInput struct {
	logger *zap.Logger
}

// NewTextInput creates a TextInput for platforms without a native field.
func NewTextInput(logger *zap.Logger) *TextInput {
	return &TextInput{logger: logger}
}

// StartHintSearchSession reports started == false so the caller falls back to
// the event tap's key stream.
func (t *TextInput) StartHintSearchSession(
	_ context.Context,
	_ ports.TextInputCallbacks,
	_ ports.TextInputFrame,
) (bool, error) {
	return false, nil
}

// StopHintSearchSession is a no-op: no session is ever started here.
func (t *TextInput) StopHintSearchSession(_ context.Context) error {
	return nil
}
