package eventtap

import (
	"context"

	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/infra/eventtap"
	"go.uber.org/zap"
)

// Adapter implements ports.EventTapPort by wrapping the existing EventTap.
type Adapter struct {
	tap     *eventtap.EventTap
	logger  *zap.Logger
	enabled bool
}

// NewAdapter creates a new event tap adapter.
func NewAdapter(tap *eventtap.EventTap, logger *zap.Logger) *Adapter {
	return &Adapter{
		tap:     tap,
		logger:  logger,
		enabled: false,
	}
}

// Enable enables the event tap.
func (a *Adapter) Enable(ctx context.Context) error {
	a.tap.Enable()
	a.enabled = true
	return nil
}

// Disable disables the event tap.
func (a *Adapter) Disable(ctx context.Context) error {
	a.tap.Disable()
	a.enabled = false
	return nil
}

// IsEnabled returns true if event capture is active.
func (a *Adapter) IsEnabled() bool {
	return a.enabled
}

// SetHandler sets the function to call when a key is pressed.
// Note: The legacy EventTap takes the callback in NewEventTap, so this is a no-op.
// The handler should be set when creating the EventTap instance.
func (a *Adapter) SetHandler(handler func(key string)) {
	// The legacy EventTap doesn't support changing the handler after creation.
	// This is a limitation of the current design.
	a.logger.Warn("SetHandler called but EventTap doesn't support changing handler after creation")
}

// Ensure Adapter implements ports.EventTapPort
var _ ports.EventTapPort = (*Adapter)(nil)
