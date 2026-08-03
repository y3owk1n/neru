//go:build windows

package grid

import (
	"image"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
)

// Overlay is the Windows grid overlay. Drawing happens in the overlay
// manager's GDI surface, so what is held here is the configuration and the
// window handle the manager draws into.
type Overlay struct {
	window unsafe.Pointer
	config config.GridConfig
	logger *zap.Logger
}

// NewOverlay creates a new grid overlay instance (Windows stub).
func NewOverlay(cfg config.GridConfig, logger *zap.Logger) (*Overlay, error) {
	return NewOverlayWithWindow(cfg, logger, nil), nil
}

// NewOverlayWithWindow creates a grid overlay instance using a shared window (Windows stub).
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

// DrawGrid draws the grid for the specified grid instance (Windows stub).
func (o *Overlay) DrawGrid(grid *domainGrid.Grid) error {
	return nil
}

// Show shows the grid overlay (Windows stub).
func (o *Overlay) Show() {}

// Hide hides the grid overlay (Windows stub).
func (o *Overlay) Hide() {}

// Destroy destroys the grid overlay (Windows stub).
func (o *Overlay) Destroy() {}

// Clear clears the grid overlay (Windows stub).
func (o *Overlay) Clear() {}

// ShowVirtualPointer is a Windows stub.
func (o *Overlay) ShowVirtualPointer(_ image.Point, _ int, _ string) {}

// HideVirtualPointer is a Windows stub.
func (o *Overlay) HideVirtualPointer() {}

// SetConfig updates the grid configuration (Windows stub).
func (o *Overlay) SetConfig(cfg config.GridConfig) {
	o.config = cfg
}

// SetVirtualPointerConfig stores the virtual pointer UI config (Windows stub).
func (o *Overlay) SetVirtualPointerConfig(_ config.VirtualPointerUI, _ string) {}

// Config returns the grid configuration (Windows stub).
func (o *Overlay) Config() config.GridConfig {
	return o.config
}

// Window returns the overlay window (Windows stub).
func (o *Overlay) Window() unsafe.Pointer {
	return o.window
}
