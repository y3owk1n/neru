//go:build linux

package grid

import (
	"image"
	"strconv"
	"strings"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
)

const (
	minFontSize  = 12
	minLineWidth = 1
	invalidColor = 0xFFFFFFFF
	hexPairCount = 2
	colorLen3    = 3
	colorLen6    = 6
	colorLen8    = 8
)

// Style holds the styling information for a grid.
type Style struct {
	LineWidth      float64
	LineColor      uint32
	LabelFontColor uint32
	LabelFontSize  float64
	LabelFontName  string
	ShowLabels     bool
}

// Overlay manages the rendering of grid overlays using native platform APIs (Linux stub).
type Overlay struct {
	window unsafe.Pointer
	config config.GridConfig
	logger *zap.Logger
}

// NewOverlay creates a new grid overlay instance (Linux stub).
func NewOverlay(cfg config.GridConfig, logger *zap.Logger) (*Overlay, error) {
	return NewOverlayWithWindow(cfg, logger, nil), nil
}

// NewOverlayWithWindow creates a grid overlay instance using a shared window (Linux stub).
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

// DrawGrid draws the grid for the specified grid instance (Linux stub).
func (o *Overlay) DrawGrid(grid *domainGrid.Grid) error {
	return nil
}

// Show shows the grid overlay (Linux stub).
func (o *Overlay) Show() {}

// Hide hides the grid overlay (Linux stub).
func (o *Overlay) Hide() {}

// Destroy destroys the grid overlay (Linux stub).
func (o *Overlay) Destroy() {}

// Clear clears the grid overlay (Linux stub).
func (o *Overlay) Clear() {}

// ShowVirtualPointer is a Linux stub.
func (o *Overlay) ShowVirtualPointer(_ image.Point, _ int, _ string) {}

// HideVirtualPointer is a Linux stub.
func (o *Overlay) HideVirtualPointer() {}

// SetConfig updates the grid configuration (Linux stub).
func (o *Overlay) SetConfig(cfg config.GridConfig) {
	o.config = cfg
}

// Config returns the grid configuration (Linux stub).
func (o *Overlay) Config() config.GridConfig {
	return o.config
}

// Window returns the overlay window (Linux stub).
func (o *Overlay) Window() unsafe.Pointer {
	return o.window
}

// BuildStyle builds the grid style from the configuration (Linux stub).
func BuildStyle(cfg config.GridConfig, theme config.ThemeProvider) Style {
	return Style{
		LineWidth: float64(max(cfg.UI.BorderWidth, minLineWidth)),
		LineColor: parseLinuxColor(
			cfg.UI.BorderColor.ForTheme(
				theme,
				config.GridBorderColorLight,
				config.GridBorderColorDark,
			),
		),
		LabelFontColor: parseLinuxColor(
			cfg.UI.TextColor.ForTheme(theme, config.GridTextColorLight, config.GridTextColorDark),
		),
		LabelFontSize: float64(max(cfg.UI.FontSize, minFontSize)),
		LabelFontName: cfg.UI.FontFamily,
		ShowLabels:    true,
	}
}

func parseLinuxColor(value string) uint32 {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	switch len(value) {
	case colorLen3:
		value = "FF" + strings.Repeat(string(value[0]), hexPairCount) +
			strings.Repeat(string(value[1]), hexPairCount) +
			strings.Repeat(string(value[2]), hexPairCount)
	case colorLen6:
		value = "FF" + value
	case colorLen8:
	default:
		return invalidColor
	}

	parsed, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		return invalidColor
	}

	return uint32(parsed)
}
