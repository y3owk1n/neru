//go:build windows

package hints

import (
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
)

// Overlay is the Windows hints overlay. Drawing happens in the overlay
// manager's GDI surface, so what is held here is the configuration and the
// window handle the manager draws into.
type Overlay struct {
	window unsafe.Pointer
	config config.HintsConfig
	logger *zap.Logger
}

// NewOverlay initializes a new hint overlay instance with its own window (Windows stub).
func NewOverlay(hintsCfg config.HintsConfig, logger *zap.Logger) (*Overlay, error) {
	return &Overlay{
		config: hintsCfg,
		logger: logger,
	}, nil
}

// NewOverlayWithWindow initializes a new hint overlay instance with an existing window (Windows stub).
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

// DrawHints draws the hints using the specified style (Windows stub).
func (o *Overlay) DrawHints(hints []*Hint, style StyleMode) error {
	return nil
}

// Show shows the hint overlay (Windows stub).
func (o *Overlay) Show() {}

// Hide hides the hint overlay (Windows stub).
func (o *Overlay) Hide() {}

// Destroy destroys the hint overlay (Windows stub).
func (o *Overlay) Destroy() {}

// Clear clears the hint overlay (Windows stub).
func (o *Overlay) Clear() {}

// SetConfig updates the hints configuration (Windows stub).
func (o *Overlay) SetConfig(cfg config.HintsConfig) {
	o.config = cfg
}

// Config returns the hints configuration (Windows stub).
func (o *Overlay) Config() config.HintsConfig {
	return o.config
}
