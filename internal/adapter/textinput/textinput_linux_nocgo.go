//go:build linux && !cgo

package textinput

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/ports"
)

// TextInput is the Linux slot without cgo, which the Wayland field needs.
// Hint search reads the event tap's key stream instead.
//
// It returns started == false rather than CodeNotSupported, because
// StartHintSearchSession is best-effort and callers already branch on started,
// so an expected degrade must not look like a failure. The text_input entry in
// ports.PlatformCapabilities reports stub here.
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

// StopHintSearchSession is a no-op, because no session is ever started here.
func (t *TextInput) StopHintSearchSession(_ context.Context) error {
	return nil
}
