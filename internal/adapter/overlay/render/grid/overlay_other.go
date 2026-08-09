//go:build !darwin

package grid

import (
	"image"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
)

// Overlay is the non-darwin grid overlay. Drawing happens in the overlay
// manager's native surface (Cairo on Linux, GDI on Windows), so what is
// held here is the configuration and the window handle the manager draws
// into.
type Overlay struct {
	window unsafe.Pointer
	config config.GridConfig
	logger *zap.Logger
}

// NewOverlay creates a new grid overlay instance (non-darwin stub).
func NewOverlay(cfg config.GridConfig, logger *zap.Logger) (*Overlay, error) {
	return NewOverlayWithWindow(cfg, logger, nil), nil
}

// NewOverlayWithWindow creates a grid overlay instance using a shared window (non-darwin stub).
func NewOverlayWithWindow(
	cfg config.GridConfig,
	logger *zap.Logger,
	windowPtr unsafe.Pointer,
) *Overlay {
	return &Overlay{
		config: cfg,
		logger: logger,
		window: windowPtr,
	}
}

// DrawGrid reports CodeNotSupported: off darwin this type owns no surface, so
// there is nothing here that can paint a grid. The manager's backend draws it.
func (o *Overlay) DrawGrid(_ *domainGrid.Grid) error {
	return derrors.New(
		derrors.CodeNotSupported,
		"grid drawing is handled by the overlay manager's native surface off darwin",
	)
}

// Show shows the grid overlay (non-darwin stub).
func (o *Overlay) Show() {}

// Hide hides the grid overlay (non-darwin stub).
func (o *Overlay) Hide() {}

// Destroy destroys the grid overlay (non-darwin stub).
func (o *Overlay) Destroy() {}

// Clear clears the grid overlay (non-darwin stub).
func (o *Overlay) Clear() {}

// ShowVirtualPointer is a non-darwin stub.
func (o *Overlay) ShowVirtualPointer(_ image.Point, _ int, _ string) {}

// HideVirtualPointer is a non-darwin stub.
func (o *Overlay) HideVirtualPointer() {}

// SetConfig updates the grid configuration (non-darwin stub).
func (o *Overlay) SetConfig(cfg config.GridConfig) {
	o.config = cfg
}

// SetVirtualPointerConfig stores the virtual pointer UI config (non-darwin stub).
func (o *Overlay) SetVirtualPointerConfig(_ config.VirtualPointerUI, _ string) {}

// Config returns the grid configuration (non-darwin stub).
func (o *Overlay) Config() config.GridConfig {
	return o.config
}

// Window returns the overlay window (non-darwin stub).
func (o *Overlay) Window() unsafe.Pointer {
	return o.window
}
