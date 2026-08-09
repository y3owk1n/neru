//go:build !darwin

package hints

import (
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// Overlay is the non-darwin hints overlay. Drawing happens in the overlay
// manager's native surface (Cairo on Linux, GDI on Windows), so what is
// held here is the configuration and the window handle the manager draws
// into.
type Overlay struct {
	window unsafe.Pointer
	config config.HintsConfig
	logger *zap.Logger
}

// NewOverlay creates a new hint overlay instance with its own window (non-darwin stub).
func NewOverlay(hintsCfg config.HintsConfig, logger *zap.Logger) (*Overlay, error) {
	return &Overlay{
		config: hintsCfg,
		logger: logger,
	}, nil
}

// NewOverlayWithWindow creates a new hint overlay instance with an existing window (non-darwin stub).
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

// DrawHints reports CodeNotSupported: off darwin this type owns no surface, so
// there is nothing here that can paint a label. The manager's backend draws
// them.
func (o *Overlay) DrawHints(_ []*Hint, _ StyleMode) error {
	return derrors.New(
		derrors.CodeNotSupported,
		"hint drawing is handled by the overlay manager's native surface off darwin",
	)
}

// Show shows the hint overlay (non-darwin stub).
func (o *Overlay) Show() {}

// Hide hides the hint overlay (non-darwin stub).
func (o *Overlay) Hide() {}

// Destroy destroys the hint overlay (non-darwin stub).
func (o *Overlay) Destroy() {}

// Clear clears the hint overlay (non-darwin stub).
func (o *Overlay) Clear() {}

// SetConfig updates the hints configuration (non-darwin stub).
func (o *Overlay) SetConfig(cfg config.HintsConfig) {
	o.config = cfg
}

// Config returns the hints configuration (non-darwin stub).
func (o *Overlay) Config() config.HintsConfig {
	return o.config
}
