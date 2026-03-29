//go:build linux

package recursivegrid

import (
	"image"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
)

// Style holds the styling information for a recursive grid.
type Style struct {
	LineWidth      float64
	LineColor      uint32
	LabelFontColor uint32
	LabelFontSize  float64
	LabelFontName  string
	ShowLabels     bool
}

// Overlay manages the rendering of recursive_grid overlays using native platform APIs (Linux stub).
type Overlay struct {
	window unsafe.Pointer
	config config.RecursiveGridConfig
	logger *zap.Logger
}

// NewOverlay creates a new recursive_grid overlay instance (Linux stub).
func NewOverlay(cfg config.RecursiveGridConfig, logger *zap.Logger) (*Overlay, error) {
	return &Overlay{
		config: cfg,
		logger: logger,
	}, nil
}

// NewOverlayWithWindow creates a recursive_grid overlay instance using a shared window (Linux stub).
func NewOverlayWithWindow(
	cfg config.RecursiveGridConfig,
	logger *zap.Logger,
	windowPtr unsafe.Pointer,
) *Overlay {
	return &Overlay{
		config: cfg,
		logger: logger,
		window: windowPtr,
	}
}

// Window returns the overlay window (Linux stub).
func (o *Overlay) Window() unsafe.Pointer {
	return o.window
}

// Config returns the recursive_grid config (Linux stub).
func (o *Overlay) Config() config.RecursiveGridConfig {
	return o.config
}

// SetConfig updates the recursive_grid configuration (Linux stub).
func (o *Overlay) SetConfig(cfg config.RecursiveGridConfig) {
	o.config = cfg
}

// SetRecursiveGridConfig updates the recursive_grid configuration (Linux stub).
func (o *Overlay) SetRecursiveGridConfig(cfg config.RecursiveGridConfig) {
	o.SetConfig(cfg)
}

// Show shows the recursive_grid overlay (Linux stub).
func (o *Overlay) Show() {}

// Hide hides the recursive_grid overlay (Linux stub).
func (o *Overlay) Hide() {}

// Destroy destroys the recursive_grid overlay (Linux stub).
func (o *Overlay) Destroy() {}

// Clear clears the recursive_grid overlay (Linux stub).
func (o *Overlay) Clear() {}

// ShowVirtualPointer is a Linux stub.
func (o *Overlay) ShowVirtualPointer(_ image.Point, _ Style) {}

// HideVirtualPointer is a Linux stub.
func (o *Overlay) HideVirtualPointer() {}

// BuildStyle builds the recursive grid style from the configuration (Linux stub).
func BuildStyle(cfg config.RecursiveGridConfig, theme config.ThemeProvider) Style {
	return Style{}
}
