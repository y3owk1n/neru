//go:build linux

package hints

import (
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
)

// Overlay is the Linux hints overlay. Drawing happens in the overlay
// manager's Cairo surface, so what is held here is the configuration and the
// window handle the manager draws into.
type Overlay struct {
	window unsafe.Pointer
	config config.HintsConfig
	logger *zap.Logger
}

// NewOverlay creates a new hint overlay instance with its own window (Linux stub).
func NewOverlay(hintsCfg config.HintsConfig, logger *zap.Logger) (*Overlay, error) {
	return &Overlay{
		config: hintsCfg,
		logger: logger,
	}, nil
}

// NewOverlayWithWindow creates a new hint overlay instance with an existing window (Linux stub).
func NewOverlayWithWindow(
	hintsCfg config.HintsConfig,
	logger *zap.Logger,
	window unsafe.Pointer,
) (*Overlay, error) {
	return &Overlay{
		config: hintsCfg,
		logger: logger,
		window: window,
	}, nil
}

// DrawHints draws the hints using the specified style (Linux stub).
func (o *Overlay) DrawHints(hints []*Hint, style StyleMode) error {
	return nil
}

// Show shows the hint overlay (Linux stub).
func (o *Overlay) Show() {}

// Hide hides the hint overlay (Linux stub).
func (o *Overlay) Hide() {}

// Destroy destroys the hint overlay (Linux stub).
func (o *Overlay) Destroy() {}

// Clear clears the hint overlay (Linux stub).
func (o *Overlay) Clear() {}

// SetConfig updates the hints configuration (Linux stub).
func (o *Overlay) SetConfig(cfg config.HintsConfig) {
	o.config = cfg
}

// Config returns the hints configuration (Linux stub).
func (o *Overlay) Config() config.HintsConfig {
	return o.config
}
